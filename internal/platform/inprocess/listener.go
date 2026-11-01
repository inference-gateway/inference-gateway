package inprocess

import (
	"context"
	"net"
	"sync"
)

const network = "inprocess"

// Listener is a net.Listener whose connections are in-memory pipes opened by
// DialContext. The gateway serves its own routes on it, so a call to itself
// never meets the network listener, its TLS or its client certificates.
type Listener struct {
	conns     chan net.Conn
	closed    chan struct{}
	closeOnce sync.Once
}

// NewListener returns an open Listener.
func NewListener() *Listener {
	return &Listener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

// Accept waits for the next DialContext and returns the server end of its pipe.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

// Close stops Accept and DialContext. Connections already handed out stay open
// until their own Close.
func (l *Listener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

// Addr names the listener, which has no network address.
func (l *Listener) Addr() net.Addr {
	return addr{}
}

// DialContext opens a pipe to the listener whatever the network and address,
// so it can stand in for an http.Transport's dialer.
func (l *Listener) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-ctx.Done():
		closePipe(server, client)
		return nil, ctx.Err()
	case <-l.closed:
		closePipe(server, client)
		return nil, net.ErrClosed
	}
}

func closePipe(server, client net.Conn) {
	_ = server.Close()
	_ = client.Close()
}

type addr struct{}

func (addr) Network() string { return network }

func (addr) String() string { return network }
