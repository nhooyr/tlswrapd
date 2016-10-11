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
	var config map[string]*proxyConfig
	if err = json.NewDecoder(f).Decode(&config); err != nil {
		log.Fatalf("error decoding config.json: %v", err)
	}
	f.Close()

	for name, pc := range config {
		go func(name string, pc *proxyConfig) {
			pc.name = name
			p, err := newProxy(pc)
			if err != nil {
				log.Make(name).Fatal(err)
			}
			p.log.Fatal(p.listenAndServe())
		}(name, pc)
	}
	runtime.Goexit()
}
