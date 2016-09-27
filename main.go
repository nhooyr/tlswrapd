package main

import (
	"encoding/json"
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
