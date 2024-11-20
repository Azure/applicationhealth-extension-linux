package main

import (
	"testing"

	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/stretchr/testify/require"
)

func Test_handlerSettingsValidate(t *testing.T) {
	// tcp includes request path
	require.Equal(t, errTcpMustNotIncludeRequestPath, handlerSettings{
		publicSettings{Protocol: "tcp", Port: 80, RequestPath: "RequestPath"},
		protectedSettings{},
	}.validate())

	// tcp without port
	require.Equal(t, errTcpConfigurationMustIncludePort, handlerSettings{
		publicSettings{Protocol: "tcp"},
		protectedSettings{},
	}.validate())

	// probe settle time cannot exceed 240 seconds
	require.Equal(t, errProbeSettleTimeExceedsThreshold, handlerSettings{
		publicSettings{Protocol: "http", IntervalInSeconds: 60, NumberOfProbes: 5},
		protectedSettings{},
	}.validate())

	require.Nil(t, handlerSettings{
		publicSettings{Protocol: "tcp", Port: 80},
		protectedSettings{},
	}.validate())

	require.Nil(t, handlerSettings{
		publicSettings{Protocol: "http", RequestPath: "healthEndpoint"},
		protectedSettings{},
	}.validate())

	require.Nil(t, handlerSettings{
		publicSettings{Protocol: "https", RequestPath: "healthEndpoint"},
		protectedSettings{},
	}.validate())

	require.Nil(t, handlerSettings{
		publicSettings{Protocol: "https", IntervalInSeconds: 30, NumberOfProbes: 3},
		protectedSettings{},
	}.validate())
}

func Test_toJSON_empty(t *testing.T) {
	s, err := toJSON(nil)
	require.Nil(t, err)
	require.Equal(t, "{}", s)
}

func Test_toJSON(t *testing.T) {
	s, err := toJSON(map[string]interface{}{
		"a": 3})
	require.Nil(t, err)
	require.Equal(t, `{"a":3}`, s)
}

func Test_unMarshalPublicSetting(t *testing.T) {

	publicSettings := map[string]interface{}{"requestPath": "health", "port": 8080, "numberOfProbes": 1, "intervalInSeconds": 5, "gracePeriod": 600, "vmWatchSettings": map[string]interface{}{"enabled": true, "globalConfigUrl": "https://testxyz.azurefd.net/config/disable-switch-config.json"}}
	h := handlerSettings{}
	err := vmextension.UnmarshalHandlerSettings(publicSettings, nil, &h.publicSettings, &h.protectedSettings)
	require.Nil(t, err)
	require.NotNil(t, h.publicSettings)
	require.Equal(t, true, h.publicSettings.VMWatchSettings.Enabled)
	require.Equal(t, "https://testxyz.azurefd.net/config/disable-switch-config.json", h.publicSettings.VMWatchSettings.GlobalConfigUrl)
}

func Test_tryGetVMWatchCohortId(t *testing.T) {
	// Test case-insensitive lookup of VMWatchCohortId
	vmWatchCohortId := "b1abbfae-ede1-4688-9499-1724268bb2b3"
	h := handlerSettings{
		publicSettings: publicSettings{
			Protocol: "tcp",
			Port:     80,
			VMWatchSettings: &vmWatchSettings{
				Enabled: true,
				EnvironmentAttributes: map[string]interface{}{
					"vmwatchcohortid": vmWatchCohortId,
				},
			},
		},
		protectedSettings: protectedSettings{},
	}
	require.Nil(t, h.validate())
	require.NotNil(t, h.vmWatchSettings())

	actualCohortId, err := h.vmWatchSettings().TryGetVMWatchCohortId()
	require.Nil(t, err)
	require.Equal(t, vmWatchCohortId, actualCohortId)

	// Test missing VMWatchCohortId
	h = handlerSettings{
		publicSettings: publicSettings{
			Protocol: "tcp",
			Port:     80,
			VMWatchSettings: &vmWatchSettings{
				Enabled: true,
				EnvironmentAttributes: map[string]interface{}{
					"key": "value",
				},
			},
		},
		protectedSettings: protectedSettings{},
	}
	actualCohortId, err = h.vmWatchSettings().TryGetVMWatchCohortId()
	require.Nil(t, err)
	require.Equal(t, "", actualCohortId)

	// Test missing EnvironmentAttributes
	h = handlerSettings{
		publicSettings: publicSettings{
			Protocol: "tcp",
			Port:     80,
			VMWatchSettings: &vmWatchSettings{
				Enabled: true,
			},
		},
		protectedSettings: protectedSettings{},
	}
	actualCohortId, err = h.vmWatchSettings().TryGetVMWatchCohortId()
	require.Nil(t, err)
	require.Equal(t, "", actualCohortId)
}
