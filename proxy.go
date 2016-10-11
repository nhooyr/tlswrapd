package main

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/nhooyr/log"
	"github.com/nhooyr/netutil"
)

// TODO better config file format and library
type proxyConfig struct {
	name   string
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

func newProxy(pc *proxyConfig) *proxy {
	return &proxy{
		bind: pc.Bind,
		dial: pc.Dial,
		log:  log.Make(pc.name),
		config: &tls.Config{
			NextProtos:         pc.Protos,
			ClientSessionCache: tls.NewLRUClientSessionCache(-1),
			MinVersion:         tls.VersionTLS12,
		},
	}
}

func (p *proxy) listenAndServe() error {
	l, err := netutil.ListenTCPKeepAlive(p.bind)
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
	// tls.DialWithDialer includes TLS handshake
	// so the timeout is significantly longer.
	Timeout:   10 * time.Second,
	KeepAlive: time.Minute,
	DualStack: true,
}

func (p *proxy) handle(c1 net.Conn) {
	p.log.Printf("accepted %v", c1.RemoteAddr())
	defer p.log.Printf("disconnected %v", c1.RemoteAddr())
	defer c1.Close()
	c2, err := tls.DialWithDialer(dialer, "tcp", p.dial, p.config)
	if err != nil {
		// TODO no tls handshake error specification so hard to distinguish errors.
		p.log.Print(err)
		return
	}
	defer c2.Close()
	err = netutil.Tunnel(c1, c2)
	if err != nil {
		p.log.Print(err)
	}
}
