package main

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"
)

type proxy struct {
	name     string
	dial     string
	// TODO optimize
	d        *net.Dialer
	protocol string
	l        *net.TCPListener
	config   *tls.Config
}

func (p *proxy) serve() error {
	var tempDelay time.Duration
	for {
		c, err := p.l.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if tempDelay > time.Second {
					tempDelay = time.Second
				}
				p.logf("accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			p.fatal(err)
		}
		tempDelay = 0
		go p.handle(c)
	}
}

func (p *proxy) handle(c1 *net.TCPConn) {
	c1.SetKeepAlive(true)
	c1.SetKeepAlivePeriod(30 * time.Second)
	raddr := c1.RemoteAddr()
	p.logf("accepted connection from %v", raddr)
	defer p.logf("disconnected connection from %v", raddr)
	c, err := p.d.Dial("tcp", p.dial)
	if err != nil {
		c1.Close()
		p.log(err)
		return
	}
	tc := c.(*net.TCPConn)
	c2 := tls.Client(c, p.config)
	err = c2.Handshake()
	if err != nil {
		c1.Close()
		p.log(err)
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, err := io.Copy(c2, c1)
		if err != nil {
			p.log(err)
		}
		tc.CloseWrite()
		c1.CloseRead()
		wg.Done()
	}()
	go func() {
		_, err = io.Copy(c1, c2)
		if err != nil {
			p.log(err)
		}
		c1.CloseWrite()
		tc.CloseRead()
		wg.Done()
	}()
	wg.Wait()
}

func (p *proxy) logf(format string, v ...interface{}) {
	logger.Printf(p.name+format, v...)
}

func (p *proxy) log(err error) {
	logger.Print(p.name, err)
}

func (p *proxy) fatal(err error) {
	logger.Fatal(p.name, err)
}
