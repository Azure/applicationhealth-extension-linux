package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
)

type probeDialAttempt struct {
	network  string
	address  string
	started  time.Time
	duration time.Duration
	done     bool
	err      error
}

type probeTrace struct {
	mu         sync.Mutex
	started    time.Time
	phase      string
	addresses  []string
	dnsErr     error
	attempts   []probeDialAttempt
	connection httptrace.GotConnInfo
	tlsVersion uint16
	cipher     uint16
}

func newProbeTrace() *probeTrace {
	return &probeTrace{started: time.Now(), phase: "TCP connect"}
}

func (p *probeTrace) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn: func(string) {
			p.mu.Lock()
			p.phase = "TCP connect"
			p.mu.Unlock()
		},
		DNSStart: func(httptrace.DNSStartInfo) {
			p.mu.Lock()
			if p.phase != "TLS handshake" && p.phase != "HTTP response" {
				p.phase = "DNS resolution"
			}
			p.mu.Unlock()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			p.mu.Lock()
			p.addresses = nil
			for _, address := range info.Addrs {
				p.addresses = append(p.addresses, address.String())
			}
			p.dnsErr = info.Err
			p.mu.Unlock()
		},
		ConnectStart: func(network, address string) {
			p.mu.Lock()
			if p.phase != "TLS handshake" && p.phase != "HTTP response" {
				p.phase = "TCP connect"
			}
			p.attempts = append(p.attempts, probeDialAttempt{
				network: network, address: address, started: time.Now(),
			})
			p.mu.Unlock()
		},
		ConnectDone: func(network, address string, err error) {
			p.mu.Lock()
			for i := range p.attempts {
				attempt := &p.attempts[i]
				if !attempt.done && attempt.network == network && attempt.address == address {
					attempt.duration = time.Since(attempt.started)
					attempt.done = true
					attempt.err = err
					break
				}
			}
			p.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			p.mu.Lock()
			p.phase = "TLS handshake"
			p.mu.Unlock()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			p.mu.Lock()
			if err == nil {
				p.tlsVersion = state.Version
				p.cipher = state.CipherSuite
			}
			p.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			p.mu.Lock()
			p.phase = "HTTP response"
			p.connection = info
			p.mu.Unlock()
		},
	}
}

// Trace callbacks may finish after an HTTP timeout. Snapshot before logging so
// the caller's logger is never used from background connection goroutines.
func (p *probeTrace) report(ctx *log.Context, detailed bool) string {
	p.mu.Lock()
	phase := p.phase
	addresses := strings.Join(p.addresses, ",")
	dnsErr := p.dnsErr
	attempts := append([]probeDialAttempt(nil), p.attempts...)
	connection := p.connection
	tlsVersion, cipher := p.tlsVersion, p.cipher
	p.mu.Unlock()
	if connection.Conn != nil && !connection.Reused {
		logProbeConnection(ctx, connection.Conn)
	}
	if detailed {
		ctx.Log("event", "probe diagnostics", "phase", phase,
			"elapsedMs", time.Since(p.started).Milliseconds(),
			"resolvedAddresses", addresses, "dnsError", dnsErr,
			"tlsVersion", fmt.Sprintf("0x%04x", tlsVersion),
			"cipherSuite", fmt.Sprintf("0x%04x", cipher))
		for _, attempt := range attempts {
			duration := attempt.duration
			if !attempt.done {
				duration = time.Since(attempt.started)
			}
			ctx.Log("event", "probe dial attempt", "network", attempt.network,
				"address", attempt.address, "completed", attempt.done,
				"elapsedMs", duration.Milliseconds(), "error", attempt.err)
		}
	}
	return phase
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
