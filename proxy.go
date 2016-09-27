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
type proxy struct {
	Bind   string   `json:"bind"`
	Dial   string   `json:"dial"`
	Protos []string `json:"protos"`

	name   string
	config *tls.Config
}

func (p *proxy) init() {
	host, _, err := net.SplitHostPort(p.Dial)
	if err != nil {
		p.fatal(err)
	}
	p.config = &tls.Config{ServerName: host, NextProtos: p.Protos}
}

func (p *proxy) listenAndServe() {
	l, err := net.Listen("tcp", p.Bind)
	if err != nil {
		p.fatal(err)
	}
	p.logf("listening on %v", l.Addr())
	l = tcpKeepAliveListener{l.(*net.TCPListener)}
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
				p.logf("%v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			p.fatal(err)
		}
		delay = 0
		go p.handle(c)
	}
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (l tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := l.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(d.KeepAlive)
	return tc, nil
}

var d = &net.Dialer{
	Timeout:   10 * time.Second, // tls.DialWithDialer includes TLS handshake.
	KeepAlive: 30 * time.Second,
	DualStack: true,
}

func (p *proxy) handle(c1 net.Conn) {
	raddr := c1.RemoteAddr()
	p.logf("accepted %v", raddr)
	defer p.logf("disconnected %v", raddr)
	c2, err := tls.DialWithDialer(d, "tcp", p.Dial, p.config)
	if err != nil {
		c1.Close()
		p.log(err)
		return
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		_, err := io.Copy(c2, c1)
		if err != nil {
			p.logf("error copying %v to %v: %v", raddr, c2.RemoteAddr(), err)
		}
		once.Do(func() {
			c2.Close()
			c1.Close()
		})
		close(done)
	}()
	_, err = io.Copy(c1, c2)
	if err != nil {
		p.logf("error copying %v to %v: %v", c2.RemoteAddr(), raddr, err)
	}
	once.Do(func() {
		c1.Close()
		c2.Close()
	})
	<-done
}

func (p *proxy) logf(format string, v ...interface{}) {
	log.Printf(p.name+format, v...)
}

func (p *proxy) log(err error) {
	log.Print(p.name, err)
}

func (p *proxy) fatal(err error) {
	log.Fatal(p.name, err)
}
