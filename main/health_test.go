package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewHttpHealthProbe(t *testing.T) {
	requestPath := "/healthcheck"

	type test struct {
		name     string
		protocol string
		port     int
		expected string
	}

	tests := []test{
		{name: "https without port", protocol: "https", expected: "https://localhost/healthcheck"},
		{name: "https with port", protocol: "https", port: 8443, expected: "https://localhost:8443/healthcheck"},
		{name: "http without port", protocol: "http", expected: "http://localhost/healthcheck"},
		{name: "http with port", protocol: "http", port: 8080, expected: "http://localhost:8080/healthcheck"},
	}

	for _, tc := range tests {
		hhp := NewHttpHealthProbe(tc.protocol, requestPath, tc.port)

		assert.Equal(t, tc.expected, hhp.Address)
	}
}
