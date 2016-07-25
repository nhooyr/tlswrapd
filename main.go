package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhooyr/log"
	"github.com/pelletier/go-toml"
)

var logger *log.Logger

func main() {
	path := flag.String("c", "/usr/local/etc/tlswrapd/config.toml", "path to the configuration file")
	timestamps := flag.Bool("timestamps", false, "enable timestamps on log lines")
	flag.Parse()

	logger = log.New(os.Stderr, *timestamps)
	tree, err := toml.LoadFile(*path)
	if err != nil {
		logger.Fatal(err)
	}

	trees, ok := tree.Get("proxies").([]*toml.TomlTree)
	if !ok {
		logger.Fatalf("%v: type of proxies is not an array of tables", tree.GetPosition("proxies"))
	}
	p := make([]*proxy, len(trees))
	var errs []string
	for i, tree := range trees {
		p[i] = new(proxy)
		v := tree.Get("name")
		if v == nil {
			errs = append(errs, missingError(tree, "name", i))
		} else {
			p[i].name, ok = v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "name", "string", i))
			} else {
				p[i].name += ": "
			}
		}
		v = tree.Get("protocol")
		if v != nil {
			p[i].protocol, ok = v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "protocol", "string", i))
			}
		}
		v = tree.Get("dial")
		if v == nil {
			errs = append(errs, missingError(tree, "dial", i))
		} else {
			p[i].dial, ok = v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "dial", "string", i))
			}
			host, _, err := net.SplitHostPort(p[i].dial)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%v: error parsing proxies[%d].dial: %v", tree.GetPosition("dial"), i, err))
			}
			p[i].config = &tls.Config{ServerName: host}
		}
		v = tree.Get("bind")
		if v == nil {
			errs = append(errs, missingError(tree, "bind", i))
		} else {
			bind, ok := v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "bind", "string", i))
			}
			laddr, err := net.ResolveTCPAddr("tcp", bind)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%v: proxies[%d].bind: %v", tree.GetPosition("bind"), i, err))
			}
			p[i].l, err = net.ListenTCP("tcp", laddr)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%v: proxies[%d].bind: %v", tree.GetPosition("bind"), i, err))
			}
		}
	}

	if errs != nil {
		for _, e := range errs {
			logger.Print(e)
		}
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		logger.Print("terminating")
		os.Exit(0)
	}()

	logger.Print("initialized")
	for i := 1; i < len(p); i++ {
		go p[i].serve()
	}
	p[0].serve()
}

func missingError(tree *toml.TomlTree, key string, i int) string {
	return fmt.Sprintf("%v: proxies[%v].%v is missing", tree.GetPosition(""), i, key)
}

func typeError(tree *toml.TomlTree, key, expectedType string, i int) string {
	return fmt.Sprintf("%v: type of proxies[%v].%v is not %v", tree.GetPosition(key), i, key, expectedType)
}