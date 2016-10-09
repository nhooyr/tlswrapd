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

func (p *proxy) init() error {
	host, _, err := net.SplitHostPort(p.Dial)
	if err != nil {
		return err
	}
	p.config = &tls.Config{ServerName: host, NextProtos: p.Protos}
	return nil
}

func (p *proxy) listenAndServe() error {
	// No KeepAlive listener because dialer uses
	// KeepAlive and the connections are proxied.
	l, err := net.Listen("tcp", p.Bind)
	if err != nil {
		return err
	}
	p.logf("listening on %v", l.Addr())
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
			return err
		}
		delay = 0
		go p.handle(c)
	}
}

var d = &net.Dialer{
	Timeout:   10 * time.Second, // tls.DialWithDialer includes TLS handshake.
	KeepAlive: time.Minute,
	DualStack: true,
}

func (p *proxy) handle(c1 net.Conn) {
	p.logf("accepted %v", c1.RemoteAddr())
	defer p.logf("disconnected %v", c1.RemoteAddr())
	c2, err := tls.DialWithDialer(d, "tcp", p.Dial, p.config)
	if err != nil {
		p.log(err)
		_ = c1.Close()
		return
	}
	first := make(chan struct{}, 1)
	var wg sync.WaitGroup
	copyClose := func(dst net.Conn, src net.Conn) {
		err := cp(dst, src)
		select {
		case first <- struct{}{}:
			if err != nil {
				p.log(err)
			}
			_ = dst.Close()
			_ = src.Close()
		default:
		}
		wg.Done()
	}
	wg.Add(2)
	go copyClose(c1, c2)
	go copyClose(c2, c1)
	wg.Wait()
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		// TODO maybe different buffer size?
		// benchmark pls
		return make([]byte, 1<<15)
	},
}

// TODO use splice on linux
// TODO move tlsmuxd and tlswrapd into single tlsproxy package.
// TODO needs some timeout to prevent torshammer ddos
func cp(dst io.Writer, src io.Reader) error {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if ew != nil {
				return ew
			}
			if nr != nw {
				return io.ErrShortWrite
			}
		}
		if er != nil {
			if er != io.EOF {
				return er
			}
			return nil
		}
	}
}

func (p *proxy) logf(format string, v ...interface{}) {
	log.Printf(p.name+format, v...)
}

func (p *proxy) log(err error) {
	log.Print(p.name, err)
}
