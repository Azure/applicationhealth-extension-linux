package main

import (
	"context"
	"fmt"
	"net"
	"net/http/httptrace"
	"strings"
	"time"
)

const (
	probeTimeout          = 30 * time.Second
	loopbackFallbackDelay = 300 * time.Millisecond
)

type loopbackDialer struct {
	dialContext   func(context.Context, string, string) (net.Conn, error)
	fallbackDelay time.Duration
	timeout       time.Duration
}

func newLoopbackDialer() *loopbackDialer {
	dialer := &net.Dialer{
		Timeout:       probeTimeout,
		KeepAlive:     30 * time.Second,
		FallbackDelay: loopbackFallbackDelay,
	}
	return &loopbackDialer{
		dialContext:   dialer.DialContext,
		fallbackDelay: loopbackFallbackDelay,
		timeout:       probeTimeout,
	}
}

func (d *loopbackDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" {
		return d.dialContext(ctx, network, address)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return d.dialContext(ctx, network, address)
	}

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	addresses := [2]string{address, ""}
	labels := [2]string{"primary " + address, "fallback not selected"}
	type result struct {
		conn   net.Conn
		err    error
		family int
		dns    *httptrace.DNSDoneInfo
	}
	results := make(chan result)
	finished := make(chan struct{})
	defer close(finished)
	// Observe the primary dial's own resolution. A separate lookup could disagree
	// with it, or consume the fallback delay before TCP dialing even starts.
	primaryContext := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSDone: func(info httptrace.DNSDoneInfo) {
			select {
			case results <- result{dns: &info}:
			case <-finished:
			}
		},
	})
	start := func(family int) {
		attemptContext := ctx
		if family == 0 {
			attemptContext = primaryContext
		}
		target := addresses[family]
		go func() {
			conn, err := d.dialContext(attemptContext, "tcp", target)
			if err != nil && conn != nil {
				conn.Close()
				conn = nil
			}
			if err == nil && conn == nil {
				err = fmt.Errorf("dial %s returned no connection", target)
			}
			select {
			case results <- result{conn: conn, err: err, family: family}:
			case <-finished:
				if conn != nil {
					conn.Close()
				}
			}
		}()
	}

	start(0)
	var timer *time.Timer
	var timerC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	resolutionSeen := false
	fallbackStarted := false
	var failures [2]error
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("loopback dial %s canceled (%s: %v; %s: %v): %w",
				address, labels[0], attemptStatus(failures[0], true),
				labels[1], attemptStatus(failures[1], fallbackStarted), ctx.Err())
		case <-timerC:
			timerC = nil
			if !fallbackStarted {
				fallbackStarted = true
				start(1)
			}
		case r := <-results:
			if r.dns != nil {
				if resolutionSeen {
					continue
				}
				resolutionSeen = true
				fallback := missingLoopbackFamily(*r.dns)
				if fallback != "" {
					addresses[1] = net.JoinHostPort(fallback, port)
					var resolvedAddresses []string
					for _, ip := range r.dns.Addrs {
						resolvedAddresses = append(resolvedAddresses, net.JoinHostPort(ip.String(), port))
					}
					labels[0] = "resolved " + strings.Join(resolvedAddresses, ",")
					labels[1] = "fallback " + addresses[1]
					timer = time.NewTimer(d.fallbackDelay)
					timerC = timer.C
				}
				continue
			}
			if r.err == nil {
				if err := ctx.Err(); err != nil {
					r.conn.Close()
					return nil, err
				}
				return r.conn, nil
			}
			failures[r.family] = r.err
			if addresses[1] == "" {
				return nil, r.err
			}
			if failures[0] != nil && failures[1] != nil {
				return nil, fmt.Errorf("loopback dial %s failed (%s: %v; %s: %w)",
					address, labels[0], failures[0], labels[1], failures[1])
			}
			if !fallbackStarted {
				timer.Stop()
				timerC = nil
				fallbackStarted = true
				start(1)
			}
		}
	}
}

func missingLoopbackFamily(info httptrace.DNSDoneInfo) string {
	if info.Err != nil {
		return ""
	}
	var ipv4, ipv6 bool
	for _, ip := range info.Addrs {
		if !ip.IP.IsLoopback() {
			return ""
		}
		if ip.IP.To4() != nil {
			ipv4 = true
		} else if ip.IP.To16() != nil {
			ipv6 = true
		}
	}
	if ipv4 && !ipv6 {
		return "::1"
	}
	if ipv6 && !ipv4 {
		return "127.0.0.1"
	}
	return ""
}

func attemptStatus(err error, started bool) string {
	if err != nil {
		return err.Error()
	}
	if started {
		return "pending"
	}
	return "not started"
}
