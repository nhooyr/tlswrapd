package main

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"
)

// TODO optimize
var d = &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

type proxy struct {
	Name     string `toml:"name"`
	Dial     string `toml:"dial"`
	Bind     string `toml:"bind"`
	Protocol string `toml:"protocol,optional"`
	l        *net.TCPListener
	config   *tls.Config
}

func (p *proxy) InitName() error {
	p.Name += ": "
	return nil
}

func (p *proxy) InitDial() error {
	host, _, err := net.SplitHostPort(p.Dial)
	if err != nil {
		return err
	}
	p.config = &tls.Config{ServerName: host}
	return nil
}

func (p *proxy) InitBind() error {
	laddr, err := net.ResolveTCPAddr("tcp", p.Bind)
	if err != nil {
		return err
	}
	p.l, err = net.ListenTCP("tcp", laddr)
	return err
}

func (p *proxy) serve() {
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
				p.logf("%v; retrying in %v", err, tempDelay)
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
	p.logf("accepted %v", raddr)
	defer p.logf("disconnected %v", raddr)
	c, err := d.Dial("tcp", p.Dial)
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
	logger.Printf(p.Name+format, v...)
}

func (p *proxy) log(err error) {
	logger.Print(p.Name, err)
}

func (p *proxy) fatal(err error) {
	logger.Fatal(p.Name, err)
}
