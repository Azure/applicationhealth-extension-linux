package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePublicSettings_port(t *testing.T) {
	err := validatePublicSettings(`{"port": "foo"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

	err = validatePublicSettings(`{"port": 0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "port: Must be greater than or equal to 1")

	err = validatePublicSettings(`{"port": 65536}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "port: Must be less than or equal to 65535")

	require.Nil(t, validatePublicSettings(`{"port": 1}`), "valid port")
	require.Nil(t, validatePublicSettings(`{"port": 65535}`), "valid port")
}

func TestValidatePublicSettings_protocol(t *testing.T) {
	err := validatePublicSettings(`{"protocol": ["foo"]}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: string, given: array")

	err = validatePublicSettings(`{"protocol": "udp"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `protocol must be one of the following: "tcp", "http", "https"`)

	require.Nil(t, validatePublicSettings(`{"protocol": "tcp"}`), "tcp protocol")
	require.Nil(t, validatePublicSettings(`{"protocol": "http"}`), "http protocol")
	require.Nil(t, validatePublicSettings(`{"protocol": "https"}`), "https protocol")
}

func TestValidatePublicSettings_requestPath(t *testing.T) {
	err := validatePublicSettings(`{"requestPath": ["foo"]}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: string, given: array")

	require.Nil(t, validatePublicSettings(`{"requestPath": ""}`), "empty string request path")
	require.Nil(t, validatePublicSettings(`{"requestPath": "health/Endpoint"}`), "valid request path")
}

func TestValidatePublicSettings_intervalInSeconds(t *testing.T) {
    err := validatePublicSettings(`{"intervalInSeconds": "foo"}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

    err = validatePublicSettings(`{"intervalInSeconds": 0}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "intervalInSeconds: Must be greater than or equal to 5")

    err = validatePublicSettings(`{"intervalInSeconds": 70}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "intervalInSeconds: Must be less than or equal to 30")

    require.Nil(t, validatePublicSettings(`{"intervalInSeconds": 5}`), "valid intervalInSeconds")
    require.Nil(t, validatePublicSettings(`{"intervalInSeconds": 20}`), "valid intervalInSeconds")
    require.Nil(t, validatePublicSettings(`{"intervalInSeconds": 60}`), "valid intervalInSeconds")
}

func TestValidatePublicSettings_numberOfProbes(t *testing.T) {
    err := validatePublicSettings(`{"numberOfProbes": "foo"}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

    err = validatePublicSettings(`{"numberOfProbes": 0}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "numberOfProbes: Must be greater than or equal to 1")

    err = validatePublicSettings(`{"numberOfProbes": 5}`)
    require.NotNil(t, err)
    require.Contains(t, err.Error(), "numberOfProbes: Must be less than or equal to 3")

    require.Nil(t, validatePublicSettings(`{"numberOfProbes": 1}`), "valid numberOfProbes")
    require.Nil(t, validatePublicSettings(`{"numberOfProbes": 2}`), "valid numberOfProbes")
    require.Nil(t, validatePublicSettings(`{"numberOfProbes": 3}`), "valid numberOfProbes")
}

func TestValidatePublicSettings_unrecognizedField(t *testing.T) {
	err := validatePublicSettings(`{"protocol": "date", "alien":0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Additional property alien is not allowed")
}

func TestValidateProtectedSettings_empty(t *testing.T) {
	require.Nil(t, validateProtectedSettings(""), "empty string")
	require.Nil(t, validateProtectedSettings("{}"), "empty string")
}

func TestValidateProtectedSettings_unrecognizedField(t *testing.T) {
	err := validateProtectedSettings(`{"alien":0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Additional property alien is not allowed")
}
