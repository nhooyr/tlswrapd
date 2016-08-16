package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhooyr/log"
	"github.com/nhooyr/toml"
)

var logger *log.Logger

type config struct {
	Proxies []*proxy
}

func main() {
	configPath := flag.String("c", "/usr/local/etc/tlswrapd/config.toml", "path to the configuration file")
	timestamps := flag.Bool("timestamps", false, "enable timestamps on log lines")
	flag.Parse()

	logger = log.New(os.Stderr, *timestamps)

	c := new(config)
	if err := toml.UnmarshalFile(*configPath, c); err != nil {
		logger.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		logger.Print("terminating")
		os.Exit(0)
	}()

	logger.Print("initialized")
	for i := 1; i < len(c.Proxies); i++ {
		go logger.Fatal(c.Proxies[i].serve())
	}
	logger.Fatal(c.Proxies[0].serve())
}
