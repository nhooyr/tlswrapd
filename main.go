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
	err = json.NewDecoder(f).Decode(&proxies)
	if err != nil {
		log.Fatalf("error decoding config.json: %v", err)
	}
	_ = f.Close()

	for name, p := range proxies {
		go func(p *proxy, name string) {
			p.name = name + ": "
			p.init()
			p.listenAndServe()
		}(p, name)
	}
	runtime.Goexit()
}
