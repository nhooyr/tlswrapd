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
	var c config
	if err = json.NewDecoder(f).Decode(&c); err != nil {
		log.Fatalf("error decoding config.json: %v", err)
	}
	f.Close()

	proxies := makeProxies(c)
	for _, p := range proxies {
		go p.listenAndServe()
	}
	runtime.Goexit()
}
