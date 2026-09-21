package main

import (
	"context"
	"errors"
	"net"
	"net/http/httptrace"
	"strings"
	"sync"
	"testing"
	"time"
)

type loopbackDialerFixture struct {
	*loopbackDialer
	dial      func(context.Context, string, string) (net.Conn, error)
	addresses []net.IPAddr
	lookupErr error
}

func resolvedLoopbackDialer(ips ...string) *loopbackDialerFixture {
	fixture := &loopbackDialerFixture{loopbackDialer: newLoopbackDialer()}
	for _, ip := range ips {
		fixture.addresses = append(fixture.addresses, net.IPAddr{IP: net.ParseIP(ip)})
	}
	fixture.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(address)
		if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
			if trace := httptrace.ContextClientTrace(ctx); trace != nil && trace.DNSDone != nil {
				trace.DNSDone(httptrace.DNSDoneInfo{Addrs: fixture.addresses, Err: fixture.lookupErr})
			}
			if fixture.lookupErr != nil {
				return nil, fixture.lookupErr
			}
		}
		return fixture.dial(ctx, network, address)
	}
	return fixture
}

func Test_loopbackDialerUsesNormalDialForBothFamiliesAndNonLoopbackMappings(t *testing.T) {
	for _, addresses := range [][]string{{"::1", "127.0.0.2"}, {"192.0.2.10"}} {
		t.Run(strings.Join(addresses, ","), func(t *testing.T) {
			dialer := resolvedLoopbackDialer(addresses...)
			expected := errors.New("normal dial result")
			calls := 0
			dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				calls++
				if address != "localhost:8081" || network != "tcp" {
					t.Errorf("configured target changed: %s %s", network, address)
				}
				return nil, expected
			}
			_, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
			if !errors.Is(err, expected) || calls != 1 {
				t.Fatalf("normal dialing changed: calls=%d, error=%v", calls, err)
			}
		})
	}
}

func Test_loopbackDialerReturnsLookupFailure(t *testing.T) {
	dialer := resolvedLoopbackDialer()
	expected := errors.New("lookup failed")
	dialer.lookupErr = expected
	dialer.dial = func(context.Context, string, string) (net.Conn, error) {
		t.Error("must not replace a failed lookup with a different target")
		return nil, expected
	}
	_, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if !errors.Is(err, expected) {
		t.Fatalf("lookup error was lost: %v", err)
	}
}

func Test_loopbackDialerStartsFallbackAndCancelsLoser(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = 20 * time.Millisecond
	dialer.timeout = time.Second
	loserCanceled := make(chan struct{})
	primaryStarted := make(chan struct{})
	winner, peer := net.Pipe()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			t.Errorf("unexpected network %s", network)
		}
		if address == "localhost:8081" {
			close(primaryStarted)
			<-ctx.Done()
			close(loserCanceled)
			return nil, ctx.Err()
		}
		if address != "127.0.0.1:8081" {
			t.Errorf("unexpected fallback address %s", address)
		}
		select {
		case <-primaryStarted:
		default:
			t.Error("IPv4 started before IPv6")
		}
		return winner, nil
	}
	start := time.Now()
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if err != nil || conn != winner {
		t.Fatalf("expected IPv4 winner, got %v, %v", conn, err)
	}
	defer conn.Close()
	if elapsed := time.Since(start); elapsed < dialer.fallbackDelay || elapsed > time.Second {
		t.Errorf("unexpected fallback timing %s", elapsed)
	}
	select {
	case <-loserCanceled:
	case <-time.After(time.Second):
		t.Fatal("losing connection attempt was not canceled")
	}
}

func Test_loopbackDialerImmediateFailureDoesNotWait(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Hour
	dialer.timeout = time.Second
	winner, peer := net.Pipe()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			return nil, errors.New("IPv6 disabled or refused")
		}
		return winner, nil
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if err != nil || conn != winner {
		t.Fatalf("IPv4 must start immediately after IPv6 failure: %v, %v", conn, err)
	}
	conn.Close()
}

func Test_loopbackDialerIPv6SuccessDoesNotStartIPv4(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Hour
	winner, peer := net.Pipe()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "LOCALHOST.:8081" {
			t.Errorf("unexpected fallback connection: %s", address)
		}
		return winner, nil
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "LOCALHOST.:8081")
	if err != nil || conn != winner {
		t.Fatalf("expected IPv6 winner: %v, %v", conn, err)
	}
	conn.Close()
}

func Test_loopbackDialerBothFailuresAreReported(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			return nil, errors.New("IPv6 refused")
		}
		return nil, errors.New("IPv4 refused")
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if conn != nil || err == nil {
		t.Fatalf("expected both-family failure: %v, %v", conn, err)
	}
	for _, detail := range []string{"IPv6 refused", "IPv4 refused", "[::1]:8081", "127.0.0.1:8081"} {
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("missing %q from error %v", detail, err)
		}
	}
}

func Test_loopbackDialerSharedDeadline(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Millisecond
	dialer.timeout = 50 * time.Millisecond
	canceled := make(chan struct{}, 2)
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		<-ctx.Done()
		canceled <- struct{}{}
		return nil, ctx.Err()
	}
	start := time.Now()
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if conn != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected shared deadline error: %v, %v", conn, err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("dial exceeded its shared deadline: %s", elapsed)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("an expired connection attempt was not canceled")
		}
	}
}

func Test_loopbackDialerCancellation(t *testing.T) {
	dialer := newLoopbackDialer()
	dialer.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		t.Error("a canceled probe must not start a connection")
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:8081")
	if conn != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v, %v", conn, err)
	}
}

type trackedProbeConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (c *trackedProbeConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { close(c.closed) })
	return err
}

func Test_loopbackDialerClosesLateSuccessfulConnection(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Millisecond
	late, latePeer := net.Pipe()
	defer latePeer.Close()
	loser := &trackedProbeConn{Conn: late, closed: make(chan struct{})}
	winner, peer := net.Pipe()
	defer peer.Close()
	releaseLoser := make(chan struct{})
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			<-releaseLoser
			return loser, nil
		}
		return winner, nil
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	close(releaseLoser)
	if err != nil || conn != winner {
		t.Fatalf("expected IPv4 winner: %v, %v", conn, err)
	}
	defer conn.Close()
	select {
	case <-loser.closed:
	case <-time.After(time.Second):
		t.Fatal("late successful connection was leaked")
	}
}

func Test_loopbackDialerPreservesExplicitAddressesAndNetworks(t *testing.T) {
	for _, test := range []struct {
		network string
		address string
	}{
		{"tcp", "127.0.0.1:8081"},
		{"tcp", "[::1]:8081"},
		{"tcp", "example.invalid:8081"},
		{"tcp4", "localhost:8081"},
		{"tcp6", "localhost:8081"},
		{"unix", "/tmp/service.sock"},
	} {
		t.Run(test.network+"/"+test.address, func(t *testing.T) {
			dialer := newLoopbackDialer()
			expected := errors.New("delegated dial")
			dialer.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				if network != test.network || address != test.address {
					t.Errorf("dial target changed: %s %s", network, address)
				}
				return nil, expected
			}
			_, err := dialer.DialContext(context.Background(), test.network, test.address)
			if !errors.Is(err, expected) {
				t.Fatalf("unexpected delegated error: %v", err)
			}
		})
	}
}
