package main

import (
	"encoding/json"
	"flag"
	"os"
	"runtime"

	"github.com/nhooyr/log"
)

func main() {
	configPath := flag.String("c", "/usr/local/etc/tlswrapd/config.json", "path to the configuration file")
	flag.Parse()
	f, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var proxies map[string]*proxy
	err = json.NewDecoder(f).Decode(&proxies)
	if err != nil {
		log.Fatal(err)
	}

	for name, p := range proxies {
		go func(p *proxy, name string) {
			p.name = name + ": "
			p.init()
			log.Printf("listening on %v", p.l.Addr())
			log.Fatal(p.serve())
		}(p, name)
	}
	runtime.Goexit()
}
