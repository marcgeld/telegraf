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

	connectorTags := map[string]string{
		"name":            "airthings_connector",
		"id":              "9990002665",
		"deviceType":      "HUB",
		"segment.id":      "68e384b9-129c-4b41-9fc3-66af3d80e7b6",
		"segment.name":    "airthings_connector",
		"segment.active":  "true",
		"segment.started": "2120-10-27T12:29:28",
	}

	connectorFields := map[string]interface{}{}
	acc.AssertContainsTaggedFields(t, "airthings_connector", connectorFields, connectorTags)

	/*
		--> air: map[]
		--> airTags: map[deviceType:HUB id:9990002665 name:airthings_connector segment.active:true segment.id:68e384b9-129c-4b41-9fc3-66af3d80e7b6 segment.name:AirthingsHub segment.started:2120-10-27T12:29:28]
		--> air: map[sample.battery:78 sample.humidity:24 sample.mold:0 sample.relayDeviceType:hub sample.rssi:-51 sample.temp:22.9 sample.voc:161]
		--> airTags: map[deviceType:WAVE_MINI id:9990019182 name:airthings_connector segment.active:true segment.id:c6ddc7f5-e052-4969-8cca-f79f6a96b4f1 segment.name:VOC segment.started:2120-09-12T07:20:28]
		--> air: map[sample.battery:100 sample.co2:1456 sample.humidity:41 sample.pressure:1000.7 sample.radonShortTermAvg:92 sample.relayDeviceType:hub sample.rssi:-64 sample.temp:19.4 sample.voc:191]
		--> airTags: map[deviceType:WAVE_PLUS id:9990131459 name:airthings_connector segment.active:true segment.id:2bd162ce-4470-429f-8eff-4680ed5c6197 segment.name:Bedroom segment.started:2122-10-22T20:19:18]
		--> air: map[sample.battery:100 sample.humidity:23 sample.radonShortTermAvg:165 sample.relayDeviceType:hub sample.rssi:-59 sample.temp:23.3]
		--> airTags: map[deviceType:WAVE_GEN2 id:9990012993 name:airthings_connector segment.active:true segment.id:3f2f2e23-f81d-46dd-8da6-9c5ed051b6e5 segment.name:Basement segment.started:2122-11-11T17:52:43]

	*/

	//acc.AssertContainsTaggedFields(t, "airthings_connector", jvmMemoryPoolFields, jvmMemoryPoolTags)
}
