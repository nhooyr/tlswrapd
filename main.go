package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/nhooyr/log"
	"github.com/nhooyr/toml"
)

func main() {
	configPath := flag.String("c", "/usr/local/etc/tlswrapd/config.toml", "path to the configuration file")
	flag.Parse()

	proxies := make(map[string]*proxy)
	if err := toml.UnmarshalFile(*configPath, &proxies); err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Print("terminating")
		os.Exit(0)
	}()

	log.Print("initialized")
	for k, p := range proxies {
		p.init(i)
		go func(k string, p *proxy) {
			p.name = k + ": "
			log.Fatal(p.serve())
		}(k, p)
	}
	runtime.Goexit()
}
