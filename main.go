package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/nhooyr/log"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Print("stopping")
		os.Exit(0)
	}()

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
			err = p.init()
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("listening on %v", p.l.Addr())
			log.Fatal(p.serve())
		}(p, name)
	}
	runtime.Goexit()
}
