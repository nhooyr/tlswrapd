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
		log.Print("terminating")
		os.Exit(0)
	}()

	configPath := flag.String("c", "/usr/local/etc/tlswrapd/config.json", "path to the configuration file")
	flag.Parse()

	proxies := make(map[string]*proxy)
	f, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(&proxies)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("initialized")
	for name, p := range proxies {
		p.name = name + ": "
		p.init()
		go func(p *proxy) {
			log.Fatal(p.serve())
		}(p)
	}
	runtime.Goexit()
}
