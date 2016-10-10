package main

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	"github.com/nhooyr/log"
)

// TODO better config file format and library
type proxyConfig struct {
	Bind   string   `json:"bind"`
	Dial   string   `json:"dial"`
	Protos []string `json:"protos"`
}

type proxy struct {
	bind   string
	dial   string
	log    log.Logger
	config *tls.Config
}

func newProxy(name string, pc *proxyConfig) *proxy {
	return &proxy{
		bind: pc.Bind,
		dial: pc.Dial,
		log:  log.Make(name + ":"),
		config: &tls.Config{
			NextProtos:         pc.Protos,
			ClientSessionCache: tls.NewLRUClientSessionCache(-1),
			MinVersion:         tls.VersionTLS12,
		},
	}
}

func (p *proxy) listenAndServe() error {
	// No KeepAlive listener because dialer uses
	// KeepAlive and the connections are proxied.
	l, err := net.Listen("tcp", p.bind)
	if err != nil {
		return err
	}
	p.log.Printf("listening on %v", l.Addr())
	return p.serve(l)
}

func (p *proxy) serve(l net.Listener) error {
	defer l.Close()
	var delay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
					if delay > time.Second {
						delay = time.Second
					}
				}
				p.log.Printf("%v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			return err
		}
		delay = 0
		go p.handle(c)
	}

}

var dialer = &net.Dialer{
	Timeout:   10 * time.Second, // tls.DialWithDialer includes TLS handshake.
	KeepAlive: time.Minute,
	DualStack: true,
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		// TODO maybe different buffer size?
		// benchmark pls
		return make([]byte, 1<<15)
	},
}

func (p *proxy) handle(c1 net.Conn) {
	p.log.Printf("accepted %v", c1.RemoteAddr())
	c2, err := tls.DialWithDialer(dialer, "tcp", p.dial, p.config)
	if err != nil {
		p.log.Print(err)
		c1.Close()
		p.log.Printf("disconnected %v", c1.RemoteAddr())
		return
	}
	first := make(chan<- struct{}, 1)
	cp := func(dst net.Conn, src net.Conn) {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		// TODO use splice on linux
		// TODO needs some timeout to prevent torshammer ddos
		_, err := io.CopyBuffer(dst, src, buf)
		select {
		case first <- struct{}{}:
			if err != nil {
				p.log.Print(err)
			}
			dst.Close()
			src.Close()
			p.log.Printf("disconnected %v", c1.RemoteAddr())
		default:
		}
	}
	go cp(c1, c2)
	cp(c2, c1)
}
