package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type HealthStatus string

const (
	Healthy   HealthStatus = "healthy"
	Unhealthy HealthStatus = "unhealthy"
	Unknown   HealthStatus = "unknown"
)

type HealthProbe interface {
	evaluate(ctx *log.Context) (HealthStatus, error)
	address() string
}

type TcpHealthProbe struct {
	Address string
}

type HttpHealthProbe struct {
	HttpClient *http.Client
	Address    string
}

func NewHealthProbe(ctx *log.Context, cfg *handlerSettings) HealthProbe {
	var p HealthProbe
	p = new(DefaultHealthProbe)

	switch cfg.protocol() {
	case "tcp":
		p = &TcpHealthProbe{
			Address: "localhost:" + strconv.Itoa(cfg.port()),
		}
		ctx.Log("event", "creating tcp probe targeting "+p.address())
	case "http":
		fallthrough
	case "https":
		p = NewHttpHealthProbe(cfg.protocol(), cfg.requestPath(), cfg.port())
		ctx.Log("event", "creating "+cfg.protocol()+" probe targeting "+p.address())
	default:
		ctx.Log("event", "default settings without probe")
	}

	return p
}

func (p *TcpHealthProbe) evaluate(ctx *log.Context) (HealthStatus, error) {
	conn, err := newLoopbackDialer().DialContext(context.Background(), "tcp", p.address())
	if err != nil {
		return Unhealthy, err
	}
	defer conn.Close()

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return Unhealthy, errUnableToConvertType
	}

	logProbeConnection(ctx, conn)
	tcpConn.SetLinger(0)
	return Healthy, nil
}

func (p *TcpHealthProbe) address() string {
	return p.Address
}

func NewHttpHealthProbe(protocol string, requestPath string, port int) *HttpHealthProbe {
	p := new(HttpHealthProbe)

	var transport *http.Transport
	if protocol == "https" {
		transport = &http.Transport{
			DialContext: newLoopbackDialer().DialContext,
			// Ignore authentication/certificate failures - just validate that the localhost
			// endpoint responds with HTTP.OK
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else if protocol == "http" {
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = newLoopbackDialer().DialContext
	}
	if transport != nil {
		p.HttpClient = &http.Client{
			CheckRedirect: noRedirect,
			Timeout:       probeTimeout,
			Transport:     transport,
		}
	}

	portString := ""
	if protocol == "http" && port != 0 && port != 80 {
		portString = ":" + strconv.Itoa(port)
	} else if protocol == "https" && port != 0 && port != 443 {
		portString = ":" + strconv.Itoa(port)
	}
	// remove first slash since we want requestPath to be defined without having to prefix with a slash
	requestPath = strings.TrimPrefix(requestPath, "/")

	p.Address = protocol + "://localhost" + portString + "/" + requestPath
	return p
}

func (p *HttpHealthProbe) evaluate(ctx *log.Context) (HealthStatus, error) {
	req, err := http.NewRequest("GET", p.address(), nil)
	if err != nil {
		return Unknown, err
	}

	req.Header.Set("User-Agent", "ApplicationHealthExtension/1.0")
	trace := newProbeTrace()
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace.clientTrace()))
	resp, err := p.HttpClient.Do(req)
	trace.report(ctx)
	if err != nil {
		// url.Error includes the request URL, which may contain sensitive query parameters.
		if requestError, ok := err.(*url.Error); ok {
			err = requestError.Err
		}
		return Unhealthy, fmt.Errorf("%s probe failed: %w", req.URL.Scheme, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return Healthy, nil
	}

	return Unhealthy, fmt.Errorf("HTTP health probe returned status %d", resp.StatusCode)
}

func (p *HttpHealthProbe) address() string {
	return p.Address
}

var (
	errNoRedirect          = errors.New("No redirect allowed")
	errUnableToConvertType = errors.New("Unable to convert type")
)

func noRedirect(req *http.Request, via []*http.Request) error {
	return errNoRedirect
}

type DefaultHealthProbe struct {
}

func (p DefaultHealthProbe) evaluate(ctx *log.Context) (HealthStatus, error) {
	return Healthy, nil
}

func (p DefaultHealthProbe) address() string {
	return ""
}
