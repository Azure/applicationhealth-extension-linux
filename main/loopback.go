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

	return d.dialLocalhost(ctx, address, port)
}

func (d *loopbackDialer) dialLocalhost(ctx context.Context, address, port string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	const (
		primaryAttempt = iota
		fallbackAttempt
	)
	type attempt struct {
		address string
		label   string
		failure error
	}
	attempts := [2]attempt{
		primaryAttempt:  {address: address, label: "primary " + address},
		fallbackAttempt: {label: "fallback not selected"},
	}
	type event struct {
		conn        net.Conn
		err         error
		attemptKind int
		dns         *httptrace.DNSDoneInfo
	}
	// Keep events unbuffered: a successful connection must not remain queued
	// after the coordinator returns. Without a receiver, the attempt instead
	// takes the finished branch and closes its connection.
	events := make(chan event)
	finished := make(chan struct{})
	defer close(finished)
	// Observe the primary dial's own resolution. A separate lookup could disagree
	// with it, or consume the fallback delay before TCP dialing even starts.
	primaryContext := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSDone: func(info httptrace.DNSDoneInfo) {
			select {
			case events <- event{dns: &info}:
			case <-finished:
			}
		},
	})
	start := func(attemptKind int) {
		attemptContext := ctx
		if attemptKind == primaryAttempt {
			attemptContext = primaryContext
		}
		target := attempts[attemptKind].address
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
			case events <- event{conn: conn, err: err, attemptKind: attemptKind}:
			case <-finished:
				// The coordinator no longer accepts results, so this attempt
				// owns cleanup of any late successful connection.
				if conn != nil {
					conn.Close()
				}
			}
		}()
	}

	start(primaryAttempt)
	var timer *time.Timer
	var timerC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	resolutionSeen := false
	fallbackStarted := false
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("loopback dial %s canceled (%s: %v; %s: %v): %w",
				address, attempts[primaryAttempt].label, attemptStatus(attempts[primaryAttempt].failure, true),
				attempts[fallbackAttempt].label, attemptStatus(attempts[fallbackAttempt].failure, fallbackStarted), ctx.Err())
		case <-timerC:
			// A nil channel disables the timer branch of select after it fires.
			timerC = nil
			if !fallbackStarted {
				fallbackStarted = true
				start(fallbackAttempt)
			}
		case e := <-events:
			if e.dns != nil {
				if resolutionSeen {
					continue
				}
				resolutionSeen = true
				fallback := missingLoopbackFamily(*e.dns)
				if fallback != "" {
					attempts[fallbackAttempt].address = net.JoinHostPort(fallback, port)
					var resolvedAddresses []string
					for _, ip := range e.dns.Addrs {
						resolvedAddresses = append(resolvedAddresses, net.JoinHostPort(ip.String(), port))
					}
					attempts[primaryAttempt].label = "resolved " + strings.Join(resolvedAddresses, ",")
					attempts[fallbackAttempt].label = "fallback " + attempts[fallbackAttempt].address
					timer = time.NewTimer(d.fallbackDelay)
					timerC = timer.C
				}
				continue
			}
			if e.err == nil {
				if err := ctx.Err(); err != nil {
					e.conn.Close()
					return nil, err
				}
				// Ownership of the winning connection passes to the caller.
				// Deferred cancellation stops the other attempt; finished makes
				// that attempt close any connection it produces after we return.
				return e.conn, nil
			}
			attempts[e.attemptKind].failure = e.err
			if attempts[fallbackAttempt].address == "" {
				return nil, e.err
			}
			if attempts[primaryAttempt].failure != nil && attempts[fallbackAttempt].failure != nil {
				return nil, fmt.Errorf("loopback dial %s failed (%s: %v; %s: %w)",
					address, attempts[primaryAttempt].label, attempts[primaryAttempt].failure,
					attempts[fallbackAttempt].label, attempts[fallbackAttempt].failure)
			}
			if !fallbackStarted {
				timer.Stop()
				// Disable the timer branch of select now that fallback starts early.
				timerC = nil
				fallbackStarted = true
				start(fallbackAttempt)
			}
		}
	}
}

// missingLoopbackFamily returns the missing canonical address only when the
// localhost lookup contains exclusively loopback addresses from exactly one family.
// Mixed-family or non-loopback results retain Go's normal dialing behavior.
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
