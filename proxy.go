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
	name   string
	Bind   string   `json:"bind"`
	Dial   string   `json:"dial"`
	Protos []string `json:"protos"`
	l      net.Listener
	config *tls.Config
}

func (p *proxy) init() error {
	host, _, err := net.SplitHostPort(p.Dial)
	if err != nil {
		return err
	}
	p.config = &tls.Config{ServerName: host, NextProtos: p.Protos}
	laddr, err := net.ResolveTCPAddr("tcp", p.Bind)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return err
	}
	p.l = tcpKeepAliveListener{l}
	return nil
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(d.KeepAlive)
	return tc, nil
}

func (p *proxy) serve() error {
	var delay time.Duration
	for {
		c, err := p.l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > time.Second {
					delay = time.Second
				}
				p.logf("%v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			return err
		}
		delay = 0
		go p.handle(c)
	}
}

var d = &net.Dialer{
	Timeout:   10 * time.Second, // tls.DialWithDialer includes tls handshake time
	KeepAlive: 30 * time.Second,
	DualStack: true,
}

// TODO maybe better logging?
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
			p.log(err)
		}
		once.Do(func() {
			c2.Close()
			c1.Close()
		})
		close(done)
	}()
	_, err = io.Copy(c1, c2)
	if err != nil {
		p.log(err)
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
