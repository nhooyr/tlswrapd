package main

import (
	"context"
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

var bufferPool = sync.Pool{
	New: func() interface{} {
		// TODO maybe different buffer size?
		// benchmark pls
		return make([]byte, 1<<15)
	},
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
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		err := cp(c1, c2, ctx, cancel)
		if err != nil {
			p.logf("error copying %v to %v: %v", c2.RemoteAddr(), c1.RemoteAddr(), err)
		}
		close(done)
	}()
	err = cp(c2, c1, ctx, cancel)
	if err != nil {
		p.logf("error copying %v to %v: %v", c1.RemoteAddr(), c2.RemoteAddr(), err)
	}
	<-done
	_ = c1.Close()
	_ = c2.Close()
}

// TODO use splice on linux
// TODO move tlsmuxd and tlswrapd into single tlsproxy package.
// TODO needs some timeout to prevent torshammer ddos
func cp(dst net.Conn, src net.Conn, ctx context.Context, cancel context.CancelFunc) error {
	b := bufferPool.Get().([]byte)
	defer bufferPool.Put(b)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			nr, er := src.Read(b)
			if nr > 0 {
				select {
				case <-ctx.Done():
					return er
				default:
					nw, ew := dst.Write(b[:nr])
					if ew != nil {
						cancel()
						return ew
					}
					if nr != nw {
						cancel()
						return io.ErrShortWrite
					}
				}
			}
			if er != nil {
				cancel()
				if er != io.EOF {
					return er
				}
				return nil
			}
		}
	}
}

func (p *proxy) logf(format string, v ...interface{}) {
	log.Printf(p.name+format, v...)
}

func (p *proxy) log(err error) {
	log.Print(p.name, err)
}
