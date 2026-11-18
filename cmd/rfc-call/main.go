package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ad3n/gonwrfc/gorfc"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	host := flag.String("host", "127.0.0.1", "RFC WebSocket host")
	port := flag.String("port", "8088", "RFC WebSocket port")
	function := flag.String("function", "STFC_CONNECTION", "RFC function")
	input := flag.String("params", `{"REQUTEXT":"Hello Lawang"}`, "JSON input parameters")
	flag.Parse()
	var params map[string]any
	if err := json.Unmarshal([]byte(*input), &params); err != nil {
		return err
	}

	conn, err := gorfc.ConnectionFromParams(gorfc.ConnectionParameters{"wshost": *host, "wsport": *port, "use_tls": "0", "user": "MOCK", "passwd": "MOCK", "client": "000", "lang": "EN"})
	if err != nil {
		return err
	}
	defer conn.Close()

	result, err := conn.Call(*function, params)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
