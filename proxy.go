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
	name  string
	Bind  string `json:"bind"`
	Dial  string `json:"dial"`
	Proto string `json:"proto"`
}

type proxy struct {
	bind   string
	dial   string
	log    log.Logger
	config *tls.Config
}

func newProxy(pc *proxyConfig) (*proxy, error) {
	host, _, err := net.SplitHostPort(pc.Dial)
	if err != nil {
		return nil, err
	}
	return &proxy{
		bind: pc.Bind,
		dial: pc.Dial,
		log:  log.Make(pc.name),
		config: &tls.Config{
			NextProtos:         []string{pc.Proto},
			ClientSessionCache: tls.NewLRUClientSessionCache(-1),
			MinVersion:         tls.VersionTLS12,
			ServerName:         host,
		},
	}, nil
}

func (p *proxy) listenAndServe() error {
	l, err := net.Listen("tcp", p.bind)
	if err != nil {
		return err
	}
	p.log.Printf("listening on %v", l.Addr())
	return p.serve(tcpKeepAliveListener{l.(*net.TCPListener)})
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (l tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := l.AcceptTCP()
	if err != nil {
		return
	}
	if err = tc.SetKeepAlive(true); err != nil {
		return
	}
	if err = tc.SetKeepAlivePeriod(time.Minute); err != nil {
		return
	}
	return tc, nil
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
	Timeout:   3 * time.Second,
	KeepAlive: time.Minute,
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 1<<16)
	},
}

func (p *proxy) handle(c1 net.Conn) {
	p.log.Printf("accepted %v", c1.RemoteAddr())
	defer p.log.Printf("disconnected %v", c1.RemoteAddr())
	defer c1.Close()
	// Not using tls.DialWithDialer because it does not
	// prefix handshake errors.
	c2, err := dialer.Dial("tcp", p.dial)
	if err != nil {
		p.log.Print(err)
		return
	}
	defer c2.Close()
	tlc := tls.Client(c2, p.config)
	if err = tlc.Handshake(); err != nil {
		// TODO should the TLS library handle prefix?
		p.log.Printf("TLS handshake error from %v: %v", c2.RemoteAddr(), err)
		return
	}
	errc := make(chan error, 2)
	cp := func(w io.Writer, r io.Reader) {
		buf := bufferPool.Get().([]byte)
		_, err := io.CopyBuffer(w, r, buf)
		errc <- err
		bufferPool.Put(buf)
	}
	go cp(struct{ io.Writer }{c1}, tlc)
	go cp(tlc, struct{ io.Reader }{c1})
	if err = <-errc; err != nil {
		p.log.Print(err)
	}
}
