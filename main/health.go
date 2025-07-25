package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"net/url"

	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/pkg/errors"
)

type HealthStatus string
type CustomMetricsStatus string

const (
	Initializing HealthStatus = "Initializing"
	Healthy      HealthStatus = "Healthy"
	Unhealthy    HealthStatus = "Unhealthy"
	Unknown      HealthStatus = "Unknown"
)

const (
	Empty string = ""
)

var (
	timeOfLastAppHealthLog = time.Time{}
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

func (p HealthStatus) GetStatusTypeForAppHealthStatus() StatusType {
	switch p {
	case Unhealthy, Unknown:
		return StatusError
	default:
		return StatusSuccess
	}
}

func (p HealthStatus) GetMessageForAppHealthStatus() string {
	if p.GetStatusTypeForAppHealthStatus() == StatusError {
		return "Application found to be unhealthy"
	} else {
		return "Application found to be healthy"
	}
}

type HealthProbe interface {
	evaluate(*slog.Logger) (ProbeResponse, error)
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

func NewHealthProbe(lg *slog.Logger, cfg *handlerSettings) HealthProbe {
	var p HealthProbe
	p = new(DefaultHealthProbe)
	switch cfg.protocol() {
	case "tcp":
		p = &TcpHealthProbe{
			Address: "localhost:" + strconv.Itoa(cfg.port()),
		}
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthProbeTask, fmt.Sprintf("Creating %s probe targeting %s", cfg.protocol(), p.address()))
	case "http":
		fallthrough
	case "https":
		p = NewHttpHealthProbe(cfg.protocol(), cfg.requestPath(), cfg.port())
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthProbeTask, fmt.Sprintf("Creating %s probe targeting %s", cfg.protocol(), p.address()))
	default:
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthProbeTask, "Configuration not provided. Using default reporting.")
	}

	return p
}

func (p *TcpHealthProbe) evaluate(lg *slog.Logger) (ProbeResponse, error) {
	conn, err := net.DialTimeout("tcp", p.address(), 30*time.Second)
	var probeResponse ProbeResponse
	if err != nil {
		probeResponse.ApplicationHealthState = Unhealthy
		return probeResponse, err
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		probeResponse.ApplicationHealthState = Unhealthy
		return probeResponse, errUnableToConvertType
	}

	tcpConn.SetLinger(0)
	tcpConn.Close()

	probeResponse.ApplicationHealthState = Healthy
	return probeResponse, nil
}

func (p *TcpHealthProbe) address() string {
	return p.Address
}

func (p *TcpHealthProbe) healthStatusAfterGracePeriodExpires() HealthStatus {
	return Unhealthy
}

// constructAddress constructs a URL string from the given protocol, port, and request path.
// If the protocol is "http" and the port is not 0 or 80, the port number is included in the URL string.
// If the protocol is "https" and the port is not 0 or 443, the port number is included in the URL string.
func constructAddress(protocol string, port int, requestPath string) string {
	portString := ""
	if protocol == "http" && port != 0 && port != 80 {
		portString = ":" + strconv.Itoa(port)
	} else if protocol == "https" && port != 0 && port != 443 {
		portString = ":" + strconv.Itoa(port)
	}

	u := url.URL{
		Scheme: protocol,
		Host:   "localhost" + portString,
		Path:   requestPath,
	}
	return u.String()
}

func NewHttpHealthProbe(protocol string, requestPath string, port int) *HttpHealthProbe {
	p := new(HttpHealthProbe)

	timeout := time.Duration(30 * time.Second)

	var transport *http.Transport
	if protocol == "https" {
		transport = &http.Transport{
			// Ignore authentication/certificate failures - just validate that the localhost
			// endpoint responds with HTTP.OK
			// MinVersion set to tls1.0 because as after go 1.18, default min version changed
			// from tls1.0 to tls1.2 and we want to support customers who are using tls1.0.
			// tls MaxVersion is set to tls1.3 by default.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // CodeQL [SM03511] Ignore TLS certificate validation, since the endpoint will be on localhost.
				MinVersion:         tls.VersionTLS10, // CodeQL [SM03566] Ignore TLS old version, since the endpoint will be on localhost.
			},
		}
		p.HttpClient = &http.Client{
			CheckRedirect: noRedirect,
			Timeout:       timeout,
			Transport:     transport,
		}
	} else {
		p.HttpClient = &http.Client{
			CheckRedirect: noRedirect,
			Timeout:       timeout,
		}
	}

	p.Address = constructAddress(protocol, port, requestPath)

	return p
}

func (p *HttpHealthProbe) evaluate(lg *slog.Logger) (ProbeResponse, error) {
	req, err := http.NewRequest("GET", p.address(), nil)
	var probeResponse ProbeResponse
	if err != nil {
		probeResponse.ApplicationHealthState = Unknown
		return probeResponse, err
	}

	req.Header.Set("User-Agent", "ApplicationHealthExtension/1.0")
	resp, err := p.HttpClient.Do(req)
	// non-2xx status code doesn't return err
	// err is returned if a timeout occurred
	if err != nil {
		probeResponse.ApplicationHealthState = Unknown
		return probeResponse, err
	}

	defer resp.Body.Close()

	// non 2xx status code
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		probeResponse.ApplicationHealthState = Unknown
		return probeResponse, errors.New(fmt.Sprintf("Unsuccessful response status code %v", resp.StatusCode))
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		probeResponse.ApplicationHealthState = Unknown
		return probeResponse, err
	}

	if err := json.Unmarshal(bodyBytes, &probeResponse); err != nil {
		probeResponse.ApplicationHealthState = Unknown
		return probeResponse, err
	}

	if err := probeResponse.validateCustomMetrics(); err != nil {
		telemetry.SendEvent(telemetry.ErrorEvent, telemetry.AppHealthProbeTask, err.Error(), "error", err)
	}

	if err := probeResponse.validateApplicationHealthState(); err != nil {
		probeResponse.ApplicationHealthState = Unknown
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

func (p DefaultHealthProbe) evaluate(lg *slog.Logger) (ProbeResponse, error) {
	var probeResponse ProbeResponse
	probeResponse.ApplicationHealthState = Healthy
	return probeResponse, nil
}

func (p DefaultHealthProbe) address() string {
	return ""
}

func (p DefaultHealthProbe) healthStatusAfterGracePeriodExpires() HealthStatus {
	return Unhealthy
}

func LogHeartBeat() {
	if time.Since(timeOfLastAppHealthLog) >= RecordAppHealthHeartBeatIntervalInMinutes*time.Minute {
		timeOfLastAppHealthLog = time.Now()
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.ReportHeatBeatTask, "AppHealthExtension is running")
	}
}
