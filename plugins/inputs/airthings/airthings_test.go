package airthings

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

const (
	HttpContentTypeKey  = "Content-Type"
	HttpContentTypeForm = "application/x-www-form-urlencoded"
	HttpContentTypeJson = "application/json"

	MesBattery           = "battery"
	MesHumidity          = "humidity"
	MesMold              = "mold"
	MesRelayDeviceType   = "relayDeviceType"
	MesRssi              = "rssi"
	MesTemp              = "temp"
	MesVoc               = "voc"
	MesRadonShortTermAvg = "radonShortTermAvg"
	MesCo2               = "co2"
	MesPressure          = "pressure"
)

func readTestData(testdataFilename string) string {
	content, err := ioutil.ReadFile(testdataFilename)
	if err != nil {
		panic(err)
	}
	return string(content)
}

// Test get mock data from device
func TestGetDeviceListAndData(t *testing.T) {
	rexp := regexp.MustCompile(`^/devices/([0-9]*)`)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var deviceId = func() string {
			devIdTmp := rexp.FindStringSubmatch(r.URL.Path)
			if devIdTmp != nil && len(devIdTmp) > 1 {
				return devIdTmp[1]
			}
			return ""
		}()

		if r.Method == http.MethodPost && r.URL.Path == "/v1/token" {
			w.Header().Set(HttpContentTypeKey, HttpContentTypeForm)
			_, err := fmt.Fprint(w, "access_token=acc35570d3n&scope=user&token_type=bearer")
			require.NoError(t, err)
		} else if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, PathDevices) {
			w.Header().Set(HttpContentTypeKey, HttpContentTypeJson)
			_, err := fmt.Fprint(w, readTestData("testdata/device_list.json"))
			require.NoError(t, err)
		} else if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/latest-samples") {
			_, serialNumber := path.Split(path.Dir(r.URL.Path))
			w.Header().Set(HttpContentTypeKey, HttpContentTypeJson)
			_, err := fmt.Fprint(w, readTestData("testdata/sample_"+serialNumber+".json"))
			require.NoError(t, err)
		} else if r.Method == http.MethodGet && len(deviceId) > 0 {
			w.Header().Set(HttpContentTypeKey, HttpContentTypeJson)
			_, err := fmt.Fprint(w, readTestData("testdata/device_"+deviceId+".json"))
			require.NoError(t, err)
		} else {
			fmt.Printf("request --> %v", r)
			_, err := fmt.Fprintln(w, readTestData("testdata/error.json"))
			require.NoError(t, err)
		}
	}))
	defer ts.Close()

	airthings := Airthings{
		URL:          ts.URL,
		ShowInactive: true,
		ClientId:     "clientid",
		ClientSecret: "clientsecret",
		TokenUrl:     ts.URL + "/v1/token",
		Scopes:       []string{"read:device:current_values"},
		Timeout:      config.Duration(5 * time.Second),
	}

	var acc testutil.Accumulator
	err := acc.GatherError(airthings.Gather)
	require.NoError(t, err)

	assertWaveMini(t, acc)
	assertWavePlus(t, acc)
	assertGen2(t, acc)
	assertHub(t, acc)
}

func assertWaveMini(t *testing.T, acc testutil.Accumulator) {
	acc.AssertContainsTaggedFields(t, "airthings_connector",
		map[string]interface{}{
			MesBattery:         float64(78),
			MesHumidity:        float64(24),
			MesMold:            float64(0),
			MesRelayDeviceType: "hub",
			MesRssi:            float64(-51),
			MesTemp:            float64(22.9),
			MesVoc:             float64(161),
		},
		map[string]string{
			TagName:           "airthings",
			TagId:             "9990019182",
			TagDeviceType:     "WAVE_MINI",
			TagSegmentId:      "c6ddc7f5-e052-4969-8cca-f79f6a96b4f1",
			TagSegmentName:    "VOC",
			TagSegmentActive:  "true",
			TagSegmentStarted: "2120-09-12T07:20:28",
		})
}

func assertWavePlus(t *testing.T, acc testutil.Accumulator) {
	acc.AssertContainsTaggedFields(t, "airthings_connector",
		map[string]interface{}{
			MesBattery:           float64(100),
			MesCo2:               float64(1456),
			MesHumidity:          float64(41),
			MesPressure:          float64(1000.7),
			MesRadonShortTermAvg: float64(92),
			MesRelayDeviceType:   "hub",
			MesRssi:              float64(-64),
			MesTemp:              float64(19.4),
			MesVoc:               float64(191),
		},
		map[string]string{
			TagDeviceType:     "WAVE_PLUS",
			TagId:             "9990131459",
			TagName:           "airthings",
			TagSegmentActive:  "true",
			TagSegmentId:      "2bd162ce-4470-429f-8eff-4680ed5c6197",
			TagSegmentName:    "Bedroom",
			TagSegmentStarted: "2122-10-22T20:19:18",
		})
}

func assertGen2(t *testing.T, acc testutil.Accumulator) {
	acc.AssertContainsTaggedFields(t, "airthings_connector",
		map[string]interface{}{
			MesBattery:           float64(100),
			MesHumidity:          float64(23),
			MesRadonShortTermAvg: float64(165),
			MesRelayDeviceType:   "hub",
			MesRssi:              float64(-59),
			MesTemp:              float64(23.3),
		},
		map[string]string{
			TagDeviceType:     "WAVE_GEN2",
			TagId:             "9990012993",
			TagName:           "airthings",
			TagSegmentActive:  "true",
			TagSegmentId:      "3f2f2e23-f81d-46dd-8da6-9c5ed051b6e5",
			TagSegmentName:    "Basement",
			TagSegmentStarted: "2122-11-11T17:52:43",
		})
}

func assertHub(t *testing.T, acc testutil.Accumulator) {
	acc.AssertContainsTaggedFields(t, "airthings_connector",
		map[string]interface{}{
			MesBattery: "N/A",
		},
		map[string]string{
			TagDeviceType:     "HUB",
			TagId:             "9990002665",
			TagName:           "airthings",
			TagSegmentActive:  "true",
			TagSegmentId:      "68e384b9-129c-4b41-9fc3-66af3d80e7b6",
			TagSegmentName:    "AirthingsHub",
			TagSegmentStarted: "2120-10-27T12:29:28",
		})
}
