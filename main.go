package main

import (
	"crypto/tls"
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
			p.config = &tls.Config{
				NextProtos:         p.Protos,
				ClientSessionCache: tls.NewLRUClientSessionCache(-1),
				MinVersion:         tls.VersionTLS12,
			}
			if err != nil {
				log.Fatalf("%s%v", p.name, err)
			}
			log.Fatalf("%s%v", p.name, p.listenAndServe())
		}(p, name)
	}
	runtime.Goexit()
}
