package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"io/ioutil"
	"encoding/json"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type HealthStatus string

const (
	Initializing	HealthStatus = "initializing"
	Healthy   		HealthStatus = "healthy"
	Draining		HealthStatus = "draining"
	Unknown			HealthStatus = "unknown"
	Disabled		HealthStatus = "disabled"
	Busy			HealthStatus = "busy"
	Unhealthy		HealthStatus = "unhealthy"
	Empty			HealthStatus = ""		
)

func (p HealthStatus) GetStatusType() StatusType {
	switch p {
	case Unknown:
		return StatusError
	default:
		return StatusSuccess 
	}
}

func (p HealthStatus) GetSubstatusMessage() string {
	return "Application found to be " + string(p)
}

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

type EndpointResponse struct {
	AppHealthState HealthStatus `json:"ApplicationHealthState"`
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
	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return Unknown, err
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Unknown, err
	}
	var endpointResponse EndpointResponse
	if err := json.Unmarshal(bodyBytes, &endpointResponse); err != nil {
		return Unknown, err
	}

	return endpointResponse.AppHealthState, nil
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
