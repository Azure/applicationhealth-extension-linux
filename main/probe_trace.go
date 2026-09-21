package main

import (
	"net"
	"net/http/httptrace"
	"sync"

	"github.com/go-kit/kit/log"
)

type probeTrace struct {
	mu         sync.Mutex
	connection httptrace.GotConnInfo
}

func newProbeTrace() *probeTrace {
	return &probeTrace{}
}

func (p *probeTrace) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			p.mu.Lock()
			p.connection = info
			p.mu.Unlock()
		},
	}
}

// Trace callbacks may finish after an HTTP timeout. Snapshot before logging so
// the caller's logger is never used from background connection goroutines.
func (p *probeTrace) report(ctx *log.Context) {
	p.mu.Lock()
	connection := p.connection
	p.mu.Unlock()
	if connection.Conn != nil && !connection.Reused {
		logProbeConnection(ctx, connection.Conn)
	}
}

func logProbeConnection(ctx *log.Context, conn net.Conn) {
	if address, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		family := "IPv6"
		if address.IP.To4() != nil {
			family = "IPv4"
		}
		ctx.Log("event", "probe connected", "family", family, "address", address.String())
	}
}
