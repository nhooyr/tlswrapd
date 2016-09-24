package main

import (
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/nhooyr/log"
	"github.com/nhooyr/toml"
)

type proxy struct {
	name string
	Bind struct {
		Val toml.NonEmptyString
		*toml.Location
	} `toml:"single"`
	Dial struct {
		Val toml.NonEmptyString
		*toml.Location
	} `toml:"single"`
	Protocols []toml.NonEmptyString
	l         *net.TCPListener
	config    *tls.Config
}

func (p *proxy) InitToml() error {
	laddr, err := net.ResolveTCPAddr("tcp", string(p.Bind.Val))
	if err != nil {
		return p.Bind.WrapError(err)
	}
	p.l, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		return p.Bind.WrapError(err)
	}
	host, _, err := net.SplitHostPort(string(p.Dial.Val))
	if err != nil {
		return p.Dial.WrapError(err)
	}
	p.config = &tls.Config{ServerName: host}
	protos := make([]string, len(p.Protocols))
	for i := range p.Protocols {
		protos[i] = string(p.Protocols[i])
	}
	p.config.NextProtos = protos
	return nil
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
				p.logf("%v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		go p.handle(c)
	}
}

// TODO optimize.
var d = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
	DualStack: true,
}

// TODO What is the compare and swap stuff in tls.Conn.Close()?
func (p *proxy) handle(tc1 *net.TCPConn) {
	raddr := tc1.RemoteAddr()
	p.logf("accepted %v", raddr)
	defer p.logf("disconnected %v", raddr)
	c, err := d.Dial("tcp", string(p.Dial))
	if err != nil {
		tc1.Close()
		p.log(err)
		return
	}
	tc2 := c.(*net.TCPConn)
	c2 := tls.Client(c, p.config)
	err = c2.Handshake()
	if err != nil {
		tc1.Close()
		p.logf("TLS handshake error from %v: %v", raddr, err)
		return
	}
	tc1.SetKeepAlive(true)
	tc1.SetKeepAlivePeriod(30 * time.Second)
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(c2, tc1)
		if err != nil {
			p.log(err)
		}
		tc2.CloseWrite()
		tc1.CloseRead()
		close(done)
	}()
	_, err = io.Copy(tc1, c2)
	if err != nil {
		p.log(err)
	}
	tc1.CloseWrite()
	tc2.CloseRead()
	<-done
}

func (p *proxy) logf(format string, v ...interface{}) {
	log.Printf(string(p.name)+format, v...)
}

func (p *proxy) log(err error) {
	log.Print(p.name, err)
}
