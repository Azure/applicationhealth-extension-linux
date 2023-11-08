package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHttpHealthProbeWithDefaultHttpPort(t *testing.T) {
	protocol := "http"
	requestPath := "/test"
	port := 80

	probe := NewHttpHealthProbe(protocol, requestPath, port)

	require.NotNil(t, probe, "Expected HttpHealthProbe, got nil")
	require.NotNil(t, probe.HttpClient, "Expected HttpClient, got nil")
	require.Equal(t, "http://localhost/test", probe.Address, "Expected address to be http://localhost/test")
}

func TestNewHttpHealthProbeWithDefaultHttpsPort(t *testing.T) {
	protocol := "https"
	requestPath := "/test"
	port := 443

	probe := NewHttpHealthProbe(protocol, requestPath, port)

	require.NotNil(t, probe, "Expected HttpHealthProbe, got nil")
	require.NotNil(t, probe.HttpClient, "Expected HttpClient, got nil")
	require.Equal(t, "https://localhost/test", probe.Address, "Expected address to be https://localhost/test")
}

func TestNewHttpHealthProbe(t *testing.T) {
	protocol := "http"
	requestPath := "/test"
	port := 8080

	probe := NewHttpHealthProbe(protocol, requestPath, port)

	require.NotNil(t, probe, "Expected HttpHealthProbe, got nil")
	require.NotNil(t, probe.HttpClient, "Expected HttpClient, got nil")
	require.Equal(t, "http://localhost:8080/test", probe.Address, "Expected address to be http://localhost:8080/test")
}

func TestConstructAddress(t *testing.T) {
	protocol := "https"
	requestPath := "/test"
	port := 8080

	address := constructAddress(protocol, port, requestPath)

	require.Equal(t, "https://localhost:8080/test", address, "Expected address to be http://localhost:8080/test")
}
