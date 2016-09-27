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
			err = p.init()
			if err != nil {
				p.fatal(err)
			}
			p.logf("listening on %v", p.l.Addr())
			p.fatal(p.serve())
		}(p, name)
	}
	runtime.Goexit()
}
