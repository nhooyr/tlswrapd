package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/nhooyr/log"
	"github.com/pelletier/go-toml"
)

func main() {
	// TODO change to real default path
	path := flag.String("c", "./config.toml", "path to configuration file")
	flag.Parse()

	tree, err := toml.LoadFile(*path)
	if err != nil {
		log.Fatal(err)
	}

	trees, ok := tree.Get("proxies").([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%v: type of proxies is not a be array of tables", tree.GetPosition("proxies"))
	}
	p := make([]*proxy, len(trees))
	var errs []error
	for i, tree := range trees {
		p[i] = new(proxy)
		v := tree.Get("name")
		if v == nil {
			errs = append(errs, missingError(tree, "name", i))
		} else {
			p[i].name, ok = v.(string)
			if !ok {
				errs = append(errs, typeError(tree, "name", "string", i))
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
				errs = append(errs, fmt.Errorf("%v: error parsing proxies[%d].dial: %v", tree.GetPosition("dial"), i, err))
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
				errs = append(errs, fmt.Errorf("%v: error resolving proxies[%d].bind: %v", tree.GetPosition("bind"), i, err))
			}
			p[i].l, err = net.ListenTCP("tcp", laddr)
			if err != nil {
				errs = append(errs, fmt.Errorf("%v: error listening on proxies[%d].bind: %v", tree.GetPosition("bind"), i, err))
			}
		}
	}
	if errs != nil {
		for _, e := range errs {
			log.Print(e)
		}
		os.Exit(1)
	}
	for i := 1; i < len(p); i++ {
		go func(i int) {
			log.Fatal(p[i].serve())
		}(i)
	}
	log.Fatal(p[0].serve())
}

func missingError(tree *toml.TomlTree, key string, i int) error {
	return fmt.Errorf("%v: proxies[%v].%v is missing", tree.GetPosition(""), i, key)
}

func typeError(tree *toml.TomlTree, key, expectedType string, i int) error {
	return fmt.Errorf("%v: type of proxies[%v].%v is not %v", tree.GetPosition(key), i, key, expectedType)
}
