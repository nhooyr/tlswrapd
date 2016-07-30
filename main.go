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

	v := tree.Get("proxies")
	if v == nil {
		logger.Fatalf("%v: missing proxies", tree.GetPosition(""))
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		logger.Fatalf("%v: proxies should be an array of tables", tree.GetPosition("proxies"))
	}
	p := make([]*proxy, len(trees))
	var errs []string
	for i, tree := range trees {
		p[i] = new(proxy)
		v = tree.Get("name")
		if v == nil {
			errs = append(errs, missingError(tree, i, "name"))
			continue
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
			errs = append(errs, missingError(tree, i, "dial"))
		} else {
			p[i].dial, ok = v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "dial", "string", i))
			} else {
				host, _, err := net.SplitHostPort(p[i].dial)
				if err != nil {
					errs = append(errs, wrapError(tree, "dial", i, err))
				} else {
					p[i].config = &tls.Config{ServerName: host}
				}
			}
		}
		v = tree.Get("bind")
		if v == nil {
			errs = append(errs, missingError(tree, i, "bind"))
		} else {
			bind, ok := v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "bind", "string", i))
			} else {
				laddr, err := net.ResolveTCPAddr("tcp", bind)
				if err != nil {
					errs = append(errs, wrapError(tree, "bind", i, err))
				} else {
					p[i].l, err = net.ListenTCP("tcp", laddr)
					if err != nil {
						errs = append(errs, wrapError(tree, "bind", i, err))
					}
				}
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

func missingError(tree *toml.TomlTree, i int, key string) string {
	return fmt.Sprintf("%v: missing proxies[%v].%v", tree.GetPosition(""), i, key)
}

func typeError(tree *toml.TomlTree, key, expectedType string, i int) string {
	return fmt.Sprintf("%v: proxies[%v].%v should be a %v", tree.GetPosition(key), i, key, expectedType)
}

func wrapError(tree *toml.TomlTree, key string, i int, err error) string {
	return fmt.Sprintf("%v: proxies[%v].%v: %v", tree.GetPosition(key), i, key, err)
}
