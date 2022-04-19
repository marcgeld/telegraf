package airthings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DeviceList struct {
	Devices []struct {
		Id         string        `json:"id"`
		DeviceType string        `json:"deviceType"`
		Sensors    []interface{} `json:"sensors"`
		Segment    struct {
			Id      string `json:"id"`
			Name    string `json:"name"`
			Started string `json:"started"`
			Active  bool   `json:"active"`
		} `json:"segment"`
		Location struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"location"`
	} `json:"devices"`
}

/*
type DeviceMeasurement struct {
	Data struct {
		Battery         int     `json:"battery"`
		Humidity        float64 `json:"humidity"`
		Mold            float64 `json:"mold"`
		Rssi            int     `json:"rssi"`
		Temp            float64 `json:"temp"`
		Time            int     `json:"time"`
		Voc             float64 `json:"voc"`
		RelayDeviceType string  `json:"relayDeviceType"`
	} `json:"data"`
}
*/

const (
	SerialNumber       = "{:serialNumber}"
	PathDevices        = "/devices"
	PathLatestSamples  = "/devices/" + SerialNumber + "/latest-samples"
	PathDevicesDetails = "/devices/" + SerialNumber
)

var sampleConfig = `
  [[inputs.airthings]]
  ## URL is the address to get metrics from
  url = "https://ext-api.airthings.com/v1/"

  ## Show inactive devices true
  showInactive = true

  ## Timeout for HTTPS
  # timeout = "5s"

  ## Interval for the Consumers API (The API is limited to 120 requests per hour)
  interval = "35s"

  ## OAuth2 Client Credentials Grant
  client_id = "<INSERT CLIENT_ID HERE>"
  client_secret = "<INSERT CLIENT_SECRET HERE>"
  # token_url = "https://accounts-api.airthings.com/v1/token"
  # scopes = ["read:device:current_values"] 

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false
`

type Airthings struct {
	URL          string          `toml:"url"`
	ShowInactive bool            `toml:"showInactive"`
	ClientId     string          `toml:"client_id"`
	ClientSecret string          `toml:"client_secret"`
	TokenUrl     string          `toml:"token_url"`
	Scopes       []string        `toml:"scopes"`
	Timeout      config.Duration `toml:"timeout"`

	tls.ClientConfig
	cfg              *clientcredentials.Config
	oAuthAccessToken *oauth2.Token // Non-concurrent security
	client           *http.Client
}

func (m *Airthings) SampleConfig() string {
	return sampleConfig
}

func (m *Airthings) Description() string {
	return "Read metrics from the devices connected to the users Airthing account"
}

func (m *Airthings) Gather(acc telegraf.Accumulator) error {
	if m.cfg == nil {
		m.cfg = &clientcredentials.Config{
			ClientID:     m.ClientId,
			ClientSecret: m.ClientSecret,
			TokenURL:     m.TokenUrl,
			Scopes:       m.Scopes,
		}
	}

	if m.oAuthAccessToken == nil {
		var err error
		m.oAuthAccessToken, err = m.cfg.Token(context.Background())
		if err != nil {
			return err
		}
	}

	if m.client == nil {
		client, err := m.createHTTPClient()
		if err != nil {
			return err
		}
		m.client = client
	}

	deviceList, err := m.deviceList()
	if err != nil {
		return err
	}
	for _, device := range deviceList.Devices {
		air := map[string]interface{}{
			"device": device,
		}

		sample, err := m.devSample(device.Id)
		if err != nil {
			return err
		}
		fmt.Printf("--> sample: %v\n\n", sample)

		details, err := m.devDetails(device.Id)
		if err != nil {
			return err
		}
		fmt.Printf("--> details: %v\n\n", details)

		for k, v := range *sample {
			//fmt.Printf("--> values: %s, %v \n", k, v)
			air[k] = v
		}
		fmt.Printf("\n")

		/*
			if rec, ok := sample.(map[string]interface{}); ok {
				for key, value := range rec {
					air[key] = value
				}
			} else {
				fmt.Printf("record not a map[string]interface{}: %v\n", record)
			}

			details, err := m.devDetails(device.Id)
			if err != nil {
				return err
			}
			if rec, ok := details.(map[string]interface{}); ok {
				for key, value := range rec {
					air[key] = value
				}
			} else {
				fmt.Printf("record not a map[string]interface{}: %v\n", record)
			}
			//fmt.Printf("--> details: %v\n", details)
		*/
		/*
			if len(sample) != 0 {
				var ts time.Time
				tsVal, ok := m["time"].(float64)
				if ok {
					tm := time.Unix(int64(tsVal), 0)
					fmt.Println(tm)
					delete(m, "time")
				}
				acc.AddFields("airthings", m, nil, ts)
			}

		*/
	}
	/*
		// add tomcat_jvm_memory measurements
		tcm := map[string]interface{}{
			"free":  status.TomcatJvm.JvmMemory.Free,
			"total": status.TomcatJvm.JvmMemory.Total,
			"max":   status.TomcatJvm.JvmMemory.Max,
		}
		acc.AddFields("tomcat_jvm_memory", tcm, nil)
	*/
	return nil
}

func (m *Airthings) devSample(deviceId string) (*map[string]interface{}, error) {
	// LatestSamples
	url := strings.Replace(PathLatestSamples, SerialNumber, deviceId, 1)
	bstr, err := m.httpRequest(url)
	if err != nil {
		return nil, err
	}
	var objmap map[string]json.RawMessage
	err = json.Unmarshal(bstr, &objmap)
	if err != nil {
		return nil, err
	}
	if dataVal, ok := objmap["data"]; ok {
		var data map[string]interface{}
		err = json.Unmarshal(dataVal, &data)
		if err != nil {
			return nil, err
		}
		return &data, nil
	}
	return nil, fmt.Errorf("No key 'data' in json data from sensor %s", deviceId)
}

func (m *Airthings) devDetails(deviceId string) (*map[string]interface{}, error) {
	url := strings.Replace(PathDevicesDetails, SerialNumber, deviceId, 1)
	bstr, err := m.httpRequest(url)
	if err != nil {
		return nil, err
	}

	var objmap map[string]interface{}
	err = json.Unmarshal(bstr, &objmap)
	if err != nil {
		return nil, err
	}
	return &objmap, nil
}

func (m *Airthings) deviceList() (DeviceList, error) {
	// DeviceList
	bodyStr, err := m.httpRequest(PathDevices)
	if err != nil {
		return DeviceList{}, err
	}

	var deviceList DeviceList
	if err := json.Unmarshal([]byte(bodyStr), &deviceList); err != nil {
		return DeviceList{}, err
	}
	return deviceList, nil
}

func (m *Airthings) httpRequest(path string) ([]byte, error) {
	var request *http.Request
	_, err := url.Parse(m.URL)
	if err != nil {
		return nil, err
	}
	request, err = http.NewRequest("GET", m.URL+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Accept", "application/json")
	m.oAuthAccessToken.SetAuthHeader(request)
	if PathDevices == path {
		query := request.URL.Query()
		query.Add("showInactive", strconv.FormatBool(m.ShowInactive))
		request.URL.RawQuery = query.Encode()
	}
	resp, err := m.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received HTTP status code %d from %q; expected 200",
			resp.StatusCode, m.URL)
	}

	buf := &bytes.Buffer{}
	buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

func (m *Airthings) createHTTPClient() (*http.Client, error) {
	tlsConfig, err := m.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: time.Duration(m.Timeout),
	}

	return client, nil
}

func init() {
	inputs.Add("airthings", func() telegraf.Input {
		return &Airthings{
			URL:          "https://ext-api.airthings.com/v1/",
			ShowInactive: true,
			ClientId:     "dummyId",
			ClientSecret: "dummySecret",
			TokenUrl:     "https://accounts-api.airthings.com/v1/token",
			Scopes:       []string{"read:device"},
			Timeout:      config.Duration(5 * time.Second),
		}
	})
}
