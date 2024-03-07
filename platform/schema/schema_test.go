package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePublicSettings_port(t *testing.T) {
	err := ValidatePublicSettings(`{"port": "foo"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

	err = ValidatePublicSettings(`{"port": 0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "port: Must be greater than or equal to 1")

	err = ValidatePublicSettings(`{"port": 65536}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "port: Must be less than or equal to 65535")

	require.Nil(t, ValidatePublicSettings(`{"port": 1}`), "valid port")
	require.Nil(t, ValidatePublicSettings(`{"port": 65535}`), "valid port")
}

func TestValidatePublicSettings_protocol(t *testing.T) {
	err := ValidatePublicSettings(`{"protocol": ["foo"]}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: string, given: array")

	err = ValidatePublicSettings(`{"protocol": "udp"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `protocol must be one of the following: "tcp", "http", "https"`)

	require.Nil(t, ValidatePublicSettings(`{"protocol": "tcp"}`), "tcp protocol")
	require.Nil(t, ValidatePublicSettings(`{"protocol": "http"}`), "http protocol")
	require.Nil(t, ValidatePublicSettings(`{"protocol": "https"}`), "https protocol")
}

func TestValidatePublicSettings_requestPath(t *testing.T) {
	err := ValidatePublicSettings(`{"requestPath": ["foo"]}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: string, given: array")

	require.Nil(t, ValidatePublicSettings(`{"requestPath": ""}`), "empty string request path")
	require.Nil(t, ValidatePublicSettings(`{"requestPath": "health/Endpoint"}`), "valid request path")
}

func TestValidatePublicSettings_intervalInSeconds(t *testing.T) {
	err := ValidatePublicSettings(`{"intervalInSeconds": "foo"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

	err = ValidatePublicSettings(`{"intervalInSeconds": 0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "intervalInSeconds: Must be greater than or equal to 5")

	err = ValidatePublicSettings(`{"intervalInSeconds": 70}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "intervalInSeconds: Must be less than or equal to 60")

	require.Nil(t, ValidatePublicSettings(`{"intervalInSeconds": 5}`), "valid intervalInSeconds")
	require.Nil(t, ValidatePublicSettings(`{"intervalInSeconds": 20}`), "valid intervalInSeconds")
	require.Nil(t, ValidatePublicSettings(`{"intervalInSeconds": 60}`), "valid intervalInSeconds")
}

func TestValidatePublicSettings_numberOfProbes(t *testing.T) {
	err := ValidatePublicSettings(`{"numberOfProbes": "foo"}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Invalid type. Expected: integer, given: string")

	err = ValidatePublicSettings(`{"numberOfProbes": 0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "numberOfProbes: Must be greater than or equal to 1")

	err = ValidatePublicSettings(`{"numberOfProbes": 25}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "numberOfProbes: Must be less than or equal to 24")

	require.Nil(t, ValidatePublicSettings(`{"numberOfProbes": 1}`), "valid numberOfProbes")
	require.Nil(t, ValidatePublicSettings(`{"numberOfProbes": 2}`), "valid numberOfProbes")
	require.Nil(t, ValidatePublicSettings(`{"numberOfProbes": 3}`), "valid numberOfProbes")
}

func TestValidatePublicSettings_unrecognizedField(t *testing.T) {
	err := ValidatePublicSettings(`{"protocol": "date", "alien":0}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Additional property alien is not allowed")
}

func TestValidateProtectedSettings_empty(t *testing.T) {
	require.Nil(t, ValidateProtectedSettings(""), "empty string")
	require.Nil(t, ValidateProtectedSettings("{}"), "empty string")
}

func TestValidateProtectedSettings_unrecognizedField(t *testing.T) {
	err := ValidateProtectedSettings(`{"alien":0}`)
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
			err := ValidatePublicSettings(tc.input)
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
	require.Nil(t, ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : false }}`), "valid settings")
	require.Nil(t, ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true }}`), "valid settings")
	require.Nil(t, ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "memoryLimitInBytes" : 30000000 }}`), "valid settings")

	err := ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "memoryLimitInBytes" : 20000000 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.memoryLimitInBytes: Must be greater than or equal to 30000000")

	err = ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "maxCpuPercentage" : 0 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.maxCpuPercentage: Must be greater than or equal to 1")

	err = ValidatePublicSettings(`{"port": 1, "vmWatchSettings" : { "enabled" : true, "maxCpuPercentage" : 101 }}`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "vmWatchSettings.maxCpuPercentage: Must be less than or equal to 100")
}
