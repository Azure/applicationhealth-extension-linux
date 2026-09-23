package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

type probeRoundTripper func(*http.Request) (*http.Response, error)

func (f probeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type probeResponseBody struct {
	io.Reader
	closed bool
}

func (b *probeResponseBody) Close() error {
	b.closed = true
	return nil
}

func Test_tcpProbeReturnsDialError(t *testing.T) {
	probe := &TcpHealthProbe{Address: "127.0.0.1:invalid-port"}
	state, err := probe.evaluate(log.NewContext(log.NewNopLogger()))
	if state != Unhealthy || err == nil {
		t.Fatalf("expected unhealthy with the dial error, got %s, %v", state, err)
	}
}

func Test_httpProbeReturnsRequestError(t *testing.T) {
	expected := errors.New("TLS handshake failed")
	probe := NewHttpHealthProbe("https", "/health?token=secret-sentinel", 8081)
	probe.HttpClient.Transport = probeRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, expected
	})
	state, err := probe.evaluate(log.NewContext(log.NewNopLogger()))
	if state != Unhealthy || !errors.Is(err, expected) {
		t.Fatalf("expected unhealthy with the underlying request error, got %s, %v", state, err)
	}
	if strings.Contains(err.Error(), "secret-sentinel") {
		t.Fatal("probe error must not expose the request query string")
	}
}

func Test_httpProbeClosesResponseBody(t *testing.T) {
	for _, statusCode := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			body := &probeResponseBody{Reader: strings.NewReader("health response")}
			probe := NewHttpHealthProbe("http", "/health", 8081)
			probe.HttpClient.Transport = probeRoundTripper(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet ||
					request.Header.Get("User-Agent") != "ApplicationHealthExtension/1.0" {
					t.Error("probe request contract changed")
				}
				return &http.Response{StatusCode: statusCode, Body: body, Header: make(http.Header)}, nil
			})
			state, _ := probe.evaluate(log.NewContext(log.NewNopLogger()))
			expected := Unhealthy
			if statusCode == http.StatusOK {
				expected = Healthy
			}
			if state != expected {
				t.Errorf("expected %s for HTTP %d, got %s", expected, statusCode, state)
			}
			if !body.closed {
				t.Error("probe response body was not closed")
			}
		})
	}
}

func Test_httpProbePreservesTLSSettings(t *testing.T) {
	probe := NewHttpHealthProbe("https", "/health", 8081)
	transport := probe.HttpClient.Transport.(*http.Transport)
	if !transport.TLSClientConfig.InsecureSkipVerify ||
		transport.TLSClientConfig.MinVersion != 0 ||
		transport.TLSClientConfig.MaxVersion != 0 ||
		len(transport.TLSClientConfig.CipherSuites) != 0 ||
		transport.ForceAttemptHTTP2 ||
		transport.TLSHandshakeTimeout != 0 {
		t.Fatal("the loopback change must preserve v1.0.7 HTTPS/TLS policy")
	}
	if probe.HttpClient.Timeout != 30*time.Second {
		t.Fatal("overall probe timeout changed")
	}
}

func Test_httpProbeDoesNotFollowRedirects(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer server.Close()
	probe := NewHttpHealthProbe("http", "/health", 8081)
	probe.Address = server.URL + "/health"
	state, err := probe.evaluate(log.NewContext(log.NewNopLogger()))
	count := atomic.LoadInt32(&requests)
	if state != Unhealthy || !errors.Is(err, errNoRedirect) || count != 1 {
		t.Fatalf("redirect semantics changed: %s, %v, requests=%d", state, err, count)
	}
}

func Test_httpProbeLogsConnectedFamily(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	probe := NewHttpHealthProbe("http", "/health", 0)
	probe.Address = server.URL + "/health"
	defer probe.HttpClient.CloseIdleConnections()
	var logs bytes.Buffer
	state, err := probe.evaluate(log.NewContext(log.NewLogfmtLogger(&logs)))
	if state != Healthy || err != nil {
		t.Fatalf("expected healthy endpoint: %s, %v", state, err)
	}
	if !strings.Contains(logs.String(), "family=IPv4") {
		t.Errorf("missing selected address family in %s", logs.String())
	}
}
