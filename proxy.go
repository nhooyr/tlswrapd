package main

import (
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/nhooyr/log"
)

type proxy struct {
	name     string
	dial     string
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
				log.Printf("Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		c.SetKeepAlive(true)
		c.SetKeepAlivePeriod(30 * time.Second)
		go p.handle(c)
	}
}

func (p *proxy) handle(c1 *net.TCPConn) {
	raddr, err := net.ResolveTCPAddr("tcp", p.dial)
	if err != nil {
		c1.Close()
		log.Print(err)
		return
	}
	c, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		c1.Close()
		log.Print(err)
		return
	}
	c.SetKeepAlive(true)
	c.SetKeepAlivePeriod(30 * time.Second)
	c2 := tls.Client(c, p.config)
	err = c2.Handshake()
	if err != nil {
		c1.Close()
		log.Print(err)
		return
	}
	go func() {
		_, err := io.Copy(c2, c1)
		if err != nil {
			log.Print(err)
		}
		c.CloseWrite()
		c1.CloseRead()
	}()
	_, err = io.Copy(c1, c2)
	if err != nil {
		log.Print(err)
	}
	c1.CloseWrite()
	c.CloseRead()
}
