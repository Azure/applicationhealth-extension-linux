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
	dial         func(context.Context, string, string) (net.Conn, error)
	addresses    []net.IPAddr
	lookupErr    error
	beforeLookup func(context.Context) error
}

type loopbackDialResult struct {
	conn net.Conn
	err  error
}

func (f *loopbackDialerFixture) startDial(ctx context.Context) <-chan loopbackDialResult {
	results := make(chan loopbackDialResult, 1)
	go func() {
		conn, err := f.DialContext(ctx, "tcp", "localhost:8081")
		results <- loopbackDialResult{conn: conn, err: err}
	}()
	return results
}

func resolvedLoopbackDialer(ips ...string) *loopbackDialerFixture {
	fixture := &loopbackDialerFixture{loopbackDialer: newLoopbackDialer()}
	for _, ip := range ips {
		fixture.addresses = append(fixture.addresses, net.IPAddr{IP: net.ParseIP(ip)})
	}
	fixture.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(address)
		if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
			if fixture.beforeLookup != nil {
				if err := fixture.beforeLookup(ctx); err != nil {
					return nil, err
				}
			}
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

func Test_loopbackDialerFallbackDelayStartsAfterLookup(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.timeout = 3 * time.Second
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	fallbackStarted := make(chan time.Time, 1)
	dialer.beforeLookup = func(ctx context.Context) error {
		close(lookupStarted)
		select {
		case <-releaseLookup:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	winner, peer := net.Pipe()
	defer winner.Close()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		fallbackStarted <- time.Now()
		return winner, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := dialer.startDial(ctx)
	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("lookup did not start")
	}
	// Keep lookup pending longer than the normal 300 ms fallback delay.
	select {
	case <-fallbackStarted:
		t.Fatal("fallback started before lookup completed")
	case result := <-results:
		t.Fatalf("dial returned before lookup completed: %v", result.err)
	case <-time.After(500 * time.Millisecond):
	}
	lookupReleasedAt := time.Now()
	close(releaseLookup)
	select {
	case startedAt := <-fallbackStarted:
		if elapsed := startedAt.Sub(lookupReleasedAt); elapsed < dialer.fallbackDelay {
			t.Errorf("lookup consumed the fallback delay: got %s, want at least %s", elapsed, dialer.fallbackDelay)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fallback did not start after lookup completed")
	}
	select {
	case result := <-results:
		if result.err != nil || result.conn != winner {
			t.Fatalf("expected fallback winner: %v, %v", result.conn, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial did not return the fallback connection")
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
	for _, test := range []struct {
		name     string
		resolved string
		fallback string
	}{
		{"IPv6-to-IPv4", "::1", "127.0.0.1:8081"},
		{"IPv4-to-IPv6", "127.0.0.1", "[::1]:8081"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dialer := resolvedLoopbackDialer(test.resolved)
			dialer.fallbackDelay = time.Hour
			dialer.timeout = time.Second
			winner, peer := net.Pipe()
			defer winner.Close()
			defer peer.Close()
			dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				if address == "localhost:8081" {
					return nil, errors.New("primary refused")
				}
				if network != "tcp" || address != test.fallback {
					t.Errorf("unexpected fallback target: %s %s", network, address)
				}
				return winner, nil
			}
			conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
			if err != nil || conn != winner {
				t.Fatalf("fallback must start immediately after primary failure: %v, %v", conn, err)
			}
		})
	}
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

func Test_loopbackDialerWaitsForPrimaryAfterFallbackFailure(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Millisecond
	dialer.timeout = time.Second
	fallbackFailed := make(chan struct{})
	releasePrimary := make(chan struct{})
	winner, peer := net.Pipe()
	defer winner.Close()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			select {
			case <-releasePrimary:
				return winner, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		close(fallbackFailed)
		return nil, errors.New("fallback refused")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := dialer.startDial(ctx)
	select {
	case <-fallbackFailed:
	case <-time.After(time.Second):
		t.Fatal("fallback did not fail")
	}
	select {
	case result := <-results:
		t.Fatalf("dial returned while primary was still pending: %v", result.err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releasePrimary)
	select {
	case result := <-results:
		if result.err != nil || result.conn != winner {
			t.Fatalf("expected primary winner after fallback failure: %v, %v", result.conn, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial did not return the primary connection")
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

func Test_loopbackDialerCancellationDuringLookup(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.timeout = time.Hour
	lookupStarted := make(chan struct{})
	lookupCanceled := make(chan struct{})
	dialStarted := make(chan string, 2)
	dialer.beforeLookup = func(ctx context.Context) error {
		close(lookupStarted)
		<-ctx.Done()
		close(lookupCanceled)
		return ctx.Err()
	}
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialStarted <- address
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := dialer.startDial(ctx)
	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("lookup did not start")
	}
	cancel()
	select {
	case result := <-results:
		if result.conn != nil || !errors.Is(result.err, context.Canceled) {
			t.Fatalf("expected caller cancellation during lookup: %v, %v", result.conn, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial did not return promptly after cancellation during lookup")
	}
	select {
	case <-lookupCanceled:
	case <-time.After(time.Second):
		t.Fatal("lookup did not receive cancellation")
	}
	select {
	case address := <-dialStarted:
		t.Fatalf("connection attempt started during canceled lookup: %s", address)
	default:
	}
}

func Test_loopbackDialerCancellationClosesLateConnection(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Millisecond
	dialer.timeout = time.Hour
	primaryStarted := make(chan struct{})
	fallbackStarted := make(chan struct{})
	primaryCanceled := make(chan struct{})
	fallbackCanceled := make(chan struct{})
	late, peer := net.Pipe()
	lateConn := &trackedProbeConn{Conn: late, closed: make(chan struct{})}
	defer lateConn.Close()
	defer peer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			close(primaryStarted)
			<-ctx.Done()
			close(primaryCanceled)
			return lateConn, nil
		}
		close(fallbackStarted)
		<-ctx.Done()
		close(fallbackCanceled)
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := dialer.startDial(ctx)
	for _, started := range []<-chan struct{}{primaryStarted, fallbackStarted} {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("both connection attempts must start before cancellation")
		}
	}
	cancel()
	select {
	case result := <-results:
		if result.conn != nil || !errors.Is(result.err, context.Canceled) {
			t.Fatalf("expected caller cancellation of active attempts: %v, %v", result.conn, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial did not return promptly after caller cancellation")
	}
	for _, signal := range []struct {
		name string
		done <-chan struct{}
	}{
		{"primary cancellation", primaryCanceled},
		{"fallback cancellation", fallbackCanceled},
		{"late connection cleanup", lateConn.closed},
	} {
		select {
		case <-signal.done:
		case <-time.After(time.Second):
			t.Fatalf("missing %s", signal.name)
		}
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

func Test_loopbackDialerPrimaryWinsAndClosesLateFallback(t *testing.T) {
	dialer := resolvedLoopbackDialer("::1")
	dialer.fallbackDelay = time.Millisecond
	dialer.timeout = time.Second
	fallbackStarted := make(chan struct{})
	fallbackCanceled := make(chan struct{})
	primary, primaryPeer := net.Pipe()
	winner := &trackedProbeConn{Conn: primary, closed: make(chan struct{})}
	defer winner.Close()
	defer primaryPeer.Close()
	fallback, fallbackPeer := net.Pipe()
	loser := &trackedProbeConn{Conn: fallback, closed: make(chan struct{})}
	defer loser.Close()
	defer fallbackPeer.Close()
	dialer.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "localhost:8081" {
			select {
			case <-fallbackStarted:
				return winner, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		close(fallbackStarted)
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Errorf("losing fallback must be canceled, not expire: %v", ctx.Err())
		}
		close(fallbackCanceled)
		return loser, nil
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:8081")
	if err != nil || conn != winner {
		t.Fatalf("expected primary winner after both attempts started: %v, %v", conn, err)
	}
	select {
	case <-fallbackCanceled:
	case <-time.After(time.Second):
		t.Fatal("losing fallback attempt was not canceled")
	}
	select {
	case <-loser.closed:
	case <-time.After(time.Second):
		t.Fatal("late fallback connection was leaked")
	}
	select {
	case <-winner.closed:
		t.Fatal("winning connection was closed before the caller could use it")
	default:
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
