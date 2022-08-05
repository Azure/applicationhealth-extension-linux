package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type HealthStatus string

const (
	Initializing HealthStatus = "Initializing"
	Healthy      HealthStatus = "Healthy"
	Unhealthy    HealthStatus = "Unhealthy"
	Unknown      HealthStatus = "Unknown"
	Empty        HealthStatus = ""
)

func (p HealthStatus) GetStatusType() StatusType {
	switch p {
	case Initializing:
		return StatusTransitioning
	case Unknown:
		return StatusError
	default:
		return StatusSuccess
	}
}

func (p HealthStatus) GetSubstatusMessage() string {
	return "Application health found to be " + strings.ToLower(string(p))
}

type HealthProbe interface {
	evaluate(ctx *log.Context) (ProbeResponse, error)
	address() string
	healthStatusAfterGracePeriodExpires() HealthStatus
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

func (p *TcpHealthProbe) evaluate(ctx *log.Context) (ProbeResponse, error) {
	conn, err := net.DialTimeout("tcp", p.address(), 30*time.Second)
	var probeResponse ProbeResponse
	if err != nil {
		probeResponse.applicationHealthState = Unhealthy
		return probeResponse, err
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		probeResponse.applicationHealthState = Unhealthy
		return probeResponse, errUnableToConvertType
	}

	tcpConn.SetLinger(0)
	tcpConn.Close()

	probeResponse.applicationHealthState = Healthy
	return probeResponse, nil
}

func (p *TcpHealthProbe) address() string {
	return p.Address
}

func (p *TcpHealthProbe) healthStatusAfterGracePeriodExpires() HealthStatus {
	return Unhealthy
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

func (p *HttpHealthProbe) evaluate(ctx *log.Context) (ProbeResponse, error) {
	req, err := http.NewRequest("GET", p.address(), nil)
	var probeResponse ProbeResponse
	if err != nil {
		probeResponse.applicationHealthState = Unknown
		return probeResponse, err
	}

	req.Header.Set("User-Agent", "ApplicationHealthExtension/1.0")
	resp, err := p.HttpClient.Do(req)
	// non-2xx status code doesn't return err
	// err is returned if a timeout occurred
	if err != nil {
		probeResponse.applicationHealthState = Unknown
		return probeResponse, err
	}

	defer resp.Body.Close()

	// non 2xx status code
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		probeResponse.applicationHealthState = Unknown
		return probeResponse, errors.New(fmt.Sprintf("Unsuccessful response status code %v", resp.StatusCode))
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		probeResponse.applicationHealthState = Unknown
		return probeResponse, err
	}

	if err := json.Unmarshal(bodyBytes, probeResponse); err != nil {
		probeResponse.applicationHealthState = Unknown
		return probeResponse, err
	} else if err := probeResponse.validate(); err != nil {
		probeResponse.applicationHealthState = Unknown
		probeResponse.customMetrics = ""
		return probeResponse, err
	}

	return probeResponse, nil
}

func (p *HttpHealthProbe) address() string {
	return p.Address
}

func (p *HttpHealthProbe) healthStatusAfterGracePeriodExpires() HealthStatus {
	return Unknown
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

func (p DefaultHealthProbe) evaluate(ctx *log.Context) (ProbeResponse, error) {
	var probeResponse ProbeResponse
	probeResponse.applicationHealthState = Healthy
	return probeResponse, nil
}

func (p DefaultHealthProbe) address() string {
	return ""
}

func (p DefaultHealthProbe) healthStatusAfterGracePeriodExpires() HealthStatus {
	return Unhealthy
}
