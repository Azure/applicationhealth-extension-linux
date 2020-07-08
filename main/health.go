package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type HealthStatus string

const (
	Healthy   HealthStatus = "healthy"
	Unhealthy HealthStatus = "unhealthy"
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
	conn, err := net.DialTimeout("tcp", p.address(), 30*time.Second)
	if err != nil {
		return Unhealthy, nil
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return Unhealthy, errUnableToConvertType
	}

	tcpConn.SetLinger(0)
	tcpConn.Close()
	return Healthy, nil
}

func (p *TcpHealthProbe) address() string {
	return p.Address
}

func NewHttpHealthProbe(protocol string, requestPath string, port int) *HttpHealthProbe {
	p := new(HttpHealthProbe)

	timeout := time.Duration(30 * time.Second)

	var transport *http.Transport
	if protocol == "https" {
		transport = &http.Transport{
			// Ignore authentication/certificate failures - just validate that the localhost
			// endpoint responds with HTTP.OK
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		p.HttpClient = &http.Client{
			CheckRedirect: noRedirect,
			Timeout:       timeout,
			Transport:     transport,
		}
	} else if protocol == "http" {
		p.HttpClient = &http.Client{
			CheckRedirect: noRedirect,
			Timeout:       timeout,
		}
	}

	portString := ""
	if protocol == "http" && port != 0 && port != 80 {
		portString = ":" + strconv.Itoa(port)
	} else if protocol == "https" && port != 0 && port != 443 {
		portString = ":" + strconv.Itoa(port)
	}
	_, length := utf8.DecodeRuneInString(s)
	// remove first slash since we want requestPath to be defined without having to prefix with a slash
	if (length > 0 && requestPath[0] == '/') {
		requestPath = requestPath[length:]
	}
	p.Address = protocol + "://localhost" + portString + "/" + requestPath
	return p
}

func (p *HttpHealthProbe) evaluate(ctx *log.Context) (HealthStatus, error) {
	req, err := http.NewRequest("GET", p.address(), nil)
	if err != nil {
		return Unhealthy, err
	}

	req.Header.Set("User-Agent", "ApplicationHealthExtension/1.0")
	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return Unhealthy, nil
	}

	if resp.StatusCode == http.StatusOK {
		return Healthy, nil
	}

	return Unhealthy, nil
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
