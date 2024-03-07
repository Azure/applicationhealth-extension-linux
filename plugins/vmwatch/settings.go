package vmwatch

// VMWatchPluginSettings holds the configuration of VMWatch plugin, It will belong as an embedded field in the publicSettings struct.
// Which will be deserialized from public configuration section of the extension handler.
// This should be in sync with publicSettingsSchema from platform/schema/schema.go.
type VMWatchPluginSettings struct {
	// VMWatchSettings holds the configuration of VMWatch plugin.
	VMWatchSettings *VMWatchSettings `json:"vmWatchSettings"`
}

type vmWatchSignalFilters struct {
	EnabledTags            []string `json:"enabledTags,array"`
	DisabledTags           []string `json:"disabledTags,array"`
	EnabledOptionalSignals []string `json:"enabledOptionalSignals,array"`
	DisabledSignals        []string `json:"disabledSignals,array"`
}

type VMWatchSettings struct {
	Enabled               bool                   `json:"enabled,boolean"`
	SignalFilters         *vmWatchSignalFilters  `json:"signalFilters"`
	ParameterOverrides    map[string]interface{} `json:"parameterOverrides,object"`
	EnvironmentAttributes map[string]interface{} `json:"environmentAttributes,object"`
	GlobalConfigUrl       string                 `json:"globalConfigUrl"`
	DisableConfigReader   bool                   `json:"disableConfigReader,boolean"`
}

func (s *VMWatchPluginSettings) GetVMWatchSettings() *VMWatchSettings {
	return s.VMWatchSettings
}
