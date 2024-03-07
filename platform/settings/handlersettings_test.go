package settings

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/plugins/apphealth"
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/stretchr/testify/require"
)

func Test_handlerSettingsValidate(t *testing.T) {
	// tcp includes request path
	require.Equal(t, apphealth.ErrTcpMustNotIncludeRequestPath,
		HandlerSettings{
			publicSettings{
				AppHealthPluginSettings: AppHealthPluginSettings{
					Protocol:    "tcp",
					Port:        80,
					RequestPath: "health",
				},
			},
			protectedSettings{},
		}.Validate())

	// tcp without port
	require.Equal(t, apphealth.ErrTcpConfigurationMustIncludePort, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "tcp"}},
		protectedSettings{},
	}.Validate())

	// probe settle time cannot exceed 240 seconds
	require.Equal(t, apphealth.ErrProbeSettleTimeExceedsThreshold, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "http", IntervalInSeconds: 60, NumberOfProbes: 5}},
		protectedSettings{},
	}.Validate())

	require.Nil(t, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "tcp", Port: 80}},
		protectedSettings{},
	}.Validate())

	require.Nil(t, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "http", RequestPath: "healthEndpoint"}},
		protectedSettings{},
	}.Validate())

	require.Nil(t, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "https", RequestPath: "healthEndpoint"}},
		protectedSettings{},
	}.Validate())

	require.Nil(t, HandlerSettings{
		publicSettings{AppHealthPluginSettings: AppHealthPluginSettings{Protocol: "https", IntervalInSeconds: 30, NumberOfProbes: 3}},
		protectedSettings{},
	}.Validate())
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
	h := HandlerSettings{}
	err := vmextension.UnmarshalHandlerSettings(publicSettings, nil, &h.publicSettings, &h.protectedSettings)
	require.Nil(t, err)
	require.NotNil(t, h.publicSettings)
	require.Equal(t, true, h.publicSettings.VMWatchSettings.Enabled)
	require.Equal(t, "https://testxyz.azurefd.net/config/disable-switch-config.json", h.publicSettings.VMWatchSettings.GlobalConfigUrl)
}

func Test_ParseAndValidateSettings(t *testing.T) {
	// Mock the logger
	logger := logging.NewExtensionLogger(nil)

	// Mock the configuration folder
	configFolder, err := os.MkdirTemp("", "config")
	require.NoError(t, err)

	// Mock RuntimeSettings

	data := map[string]interface{}{
		"runtimeSettings": []map[string]interface{}{
			{
				"handlerSettings": map[string]interface{}{
					"publicSettings": map[string]interface{}{
						"protocol":          "http",
						"port":              80,
						"requestPath":       "health",
						"intervalInSeconds": 5,
						"numberOfProbes":    1,
						"gracePeriod":       600,
						"vmWatchSettings": map[string]interface{}{
							"enabled":         true,
							"globalConfigUrl": "https://testxyz.azurefd.net/config/disable-switch-config.json",
						},
					},
				},
			},
		},
	}

	// Marshal the map into JSON
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	err = os.WriteFile(configFolder+"/0.settings", jsonData, 0644)
	require.NoError(t, err)

	// Mock the expected handler settings
	expectedSettings := HandlerSettings{
		publicSettings: publicSettings{
			AppHealthPluginSettings: AppHealthPluginSettings{
				Protocol:          "http",
				Port:              80,
				RequestPath:       "health",
				IntervalInSeconds: 5,
				NumberOfProbes:    1,
				GracePeriod:       600,
			},
			VMWatchPluginSettings: VMWatchPluginSettings{
				VMWatchSettings: &VMWatchSettings{
					Enabled:         true,
					GlobalConfigUrl: "https://testxyz.azurefd.net/config/disable-switch-config.json",
				},
			},
		},
		protectedSettings: protectedSettings{},
	}

	// Call the ParseAndValidateSettings function
	settings, err := ParseAndValidateSettings(logger, configFolder)
	require.NoError(t, err)

	require.NoError(t, settings.Validate(), "Settings validation failed")

	// Verify the results
	require.NoError(t, err)
	require.Equal(t, expectedSettings, settings)
}
