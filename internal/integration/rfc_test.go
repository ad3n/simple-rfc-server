package integration

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ad3n/gonwrfc/gorfc"
)

func TestRFC(t *testing.T) {
	if os.Getenv("RFC_MOCK_INTEGRATION") != "1" {
		t.Skip("start the mock server and set RFC_MOCK_INTEGRATION=1")
	}

	port := os.Getenv("RFC_MOCK_PORT")
	if port == "" {
		port = "8088"
	}

	uri, err := url.Parse("rfc:///?wshost=127.0.0.1&wsport=" + port + "&use_tls=0&lang=EN")
	if err != nil {
		t.Fatal(err)
	}

	params := gorfc.ConnectionParameters{"user": "MOCK", "passwd": "MOCK", "client": "000"}
	for key, values := range uri.Query() {
		params[key] = values[0]
	}

	params["ashost"] = uri.Hostname()
	params["sysnr"] = uri.Port()
	conn, err := gorfc.ConnectionFromParams(params)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}

	description, err := conn.GetFunctionDescription("STFC_CONNECTION")
	if err != nil {
		t.Fatal(err)
	}

	if len(description.Parameters) != 3 {
		t.Fatalf("parameters = %d", len(description.Parameters))
	}

	for _, input := range []string{"Hello Lawang", "Halo 世界", strings.Repeat("A", 255), ""} {
		output, err := conn.Call("STFC_CONNECTION", map[string]any{"REQUTEXT": input})
		if err != nil {
			t.Fatal(err)
		}

		if output["ECHOTEXT"] != input || output["RESPTEXT"] != "Hello from RFC mock Lawang-Core" {
			t.Fatalf("unexpected echo: %#v", output)
		}
	}

	output, err := conn.Call("Z_LAWANG_TEST", map[string]any{"IV_NAME": "Lawang", "CV_COUNTER": 42})
	if err != nil {
		t.Fatal(err)
	}

	if output["EV_NAME"] != "Lawang" || output["EV_STATUS"] != "SUCCESS" || output["EV_COUNT"] != int32(3) || output["EV_RATIO"] != float64(1.25) || output["CV_COUNTER"] != int32(42) {
		t.Fatalf("unexpected custom output: %#v", output)
	}

	if _, err := conn.Call("Z_UNKNOWN_FUNCTION", nil); err == nil {
		t.Fatal("unknown function should fail")
	}
}
