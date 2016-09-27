package main

import (
	"encoding/json"
	"os"
	"runtime"

	"github.com/nhooyr/log"
)

func main() {
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}

	var proxies map[string]*proxy
	err = json.NewDecoder(f).Decode(&proxies)
	if err != nil {
		log.Fatalf("error decoding config.json: %v", err)
	}

	for name, p := range proxies {
		go func(p *proxy, name string) {
			p.name = name + ": "
			p.init()
			p.listenAndServe()
		}(p, name)
	}
	runtime.Goexit()
}
