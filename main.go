package main

import (
	"encoding/json"
	"flag"
	"os"
	"runtime"

	"github.com/nhooyr/log"
)

func main() {
	configPath := flag.String("c", "", "path to configuration file")
	flag.Parse()

	f, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	var proxies map[string]*proxy
	if err = json.NewDecoder(f).Decode(&proxies); err != nil {
		log.Fatalf("error decoding config.json: %v", err)
	}
	f.Close()

	for name, p := range proxies {
		go p.run(name)
	}
	runtime.Goexit()
}
