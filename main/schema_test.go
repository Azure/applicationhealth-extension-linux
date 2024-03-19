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
	require.Contains(t, err.Error(), "intervalInSeconds: Must be less than or equal to 60")

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

	err = validatePublicSettings(`{"numberOfProbes": 25}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "numberOfProbes: Must be less than or equal to 24")

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

func TestValidatePublicSettings_gracePeriod(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "invalid type",
			input:       `{"gracePeriod": "foo"}`,
			expectedErr: "Invalid type. Expected: integer, given: string",
		},
		{
			name:        "invalid value (equal to 0)",
			input:       `{"gracePeriod": 0}`,
			expectedErr: "gracePeriod: Must be greater than or equal to 5",
		},
		{
			name:        "invalid value (less than min value)",
			input:       `{"gracePeriod": 4}`,
			expectedErr: "gracePeriod: Must be greater than or equal to 5",
		},
		{
			name:        "invalid value (greater than max value)",
			input:       `{"gracePeriod": 15000}`,
			expectedErr: "gracePeriod: Must be less than or equal to 14400",
		},
		{
			name:        "valid value (equal to min value)",
			input:       `{"gracePeriod": 5}`,
			expectedErr: "",
		},
		{
			name:        "valid value (between min and max value)",
			input:       `{"gracePeriod": 7201}`,
			expectedErr: "",
		},
		{
			name:        "valid value (equal to max value)",
			input:       `{"gracePeriod": 14400}`,
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePublicSettings(tc.input)
			if tc.expectedErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}

func TestValidatePublicSettings_vmwatch(t *testing.T) {
	require.Nil(t, validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : false }}`), "valid settings")
	require.Nil(t, validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true }}`), "valid settings")
	require.Nil(t, validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "memoryLimitInBytes" : 30000000 }}`), "valid settings")

	err := validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "memoryLimitInBytes" : 20000000 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.memoryLimitInBytes: Must be greater than or equal to 30000000")

	err = validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "maxCpuPercentage" : 0 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.maxCpuPercentage: Must be greater than or equal to 1")

	err = validatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "maxCpuPercentage" : 101 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.maxCpuPercentage: Must be less than or equal to 100")
}

