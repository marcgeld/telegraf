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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		} else if r.Method == http.MethodGet && func(matched bool, err error) bool {
			if err != nil {
				return matched
			}
			return false
		}(regexp.MatchString(`^/devices/\d+$`, "aaxbb")) {
			_, serialNumber := path.Split(path.Dir(r.URL.Path))
			w.Header().Set(HttpContentTypeKey, HttpContentTypeJson)
			_, err := fmt.Fprint(w, readTestData("testdata/sample_"+serialNumber+".json"))
			require.NoError(t, err)
		} else {
			fmt.Printf("--> %v", r)
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

	/*inputs.Add("airthings", func() telegraf.Input {
		return &airthings
	})*/

	var acc testutil.Accumulator
	require.NoError(t, airthings.Gather(&acc))

	/*
		// tomcat_jvm_memory
		jvmMemoryFields := map[string]interface{}{
			"free":  int64(17909336),
			"total": int64(58195968),
			"max":   int64(620756992),
		}
		acc.AssertContainsFields(t, "tomcat_jvm_memory", jvmMemoryFields)
	*/
	/*
		connectorTags := map[string]string{
			"name": "http-apr-8080",
		}
		acc.AssertContainsTaggedFields(t, "tomcat_connector", connectorFields, connectorTags)
		*

		/*
			tc := Tomcat{
				URL:      ts.URL,
				Username: "tomcat",
				Password: "s3cret",
			}

			var acc testutil.Accumulator
			require.NoError(t, tc.Gather(&acc))


			acc.AssertContainsTaggedFields(t, "tomcat_jvm_memorypool", jvmMemoryPoolFields, jvmMemoryPoolTags)



	*/
}
