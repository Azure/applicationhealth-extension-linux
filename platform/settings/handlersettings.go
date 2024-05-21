package settings

import (
	"encoding/json"

	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/platform/schema"
	"github.com/Azure/applicationhealth-extension-linux/plugins/apphealth"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/pkg/errors"
)

// Aliases for the plugin settings
type AppHealthPluginSettings = apphealth.AppHealthPluginSettings
type VMWatchPluginSettings = vmwatch.VMWatchPluginSettings
type VMWatchSettings = vmwatch.VMWatchSettings

// HandlerSettings holds the configuration of the extension handler.
type HandlerSettings struct {
	publicSettings
	protectedSettings
}

func (s HandlerSettings) String() string {
	settings, _ := json.MarshalIndent(s, "", "    ")
	return string(settings)
}

// publicSettings is the type deserialized from public configuration section of
// the extension handler. This should be in sync with publicSettingsSchema.
//   - AppHealthPluginSettings holds the configuration of the AppHealth plugin
//   - VMWatchPluginSettings holds the configuration of VMWatch plugin
type publicSettings struct {
	AppHealthPluginSettings
	VMWatchPluginSettings
}

// protectedSettings is the type decoded and deserialized from protected
// configuration section. This should be in sync with protectedSettingsSchema.
type protectedSettings struct {
}

// ParseAndValidateSettings reads configuration from configFolder, decrypts it,
// runs JSON-schema and logical validation on it and returns it back.
func ParseAndValidateSettings(lg logging.Logger, configFolder string) (h HandlerSettings, _ error) {
	lg.Info("reading configuration")
	pubJSON, protJSON, err := readSettings(configFolder)
	if err != nil {
		return h, err
	}
	lg.Info("read configuration")

	lg.Info("validating json schema")
	if err := validateSettingsSchema(pubJSON, protJSON); err != nil {
		return h, errors.Wrap(err, "json validation error")
	}
	lg.Info("json schema valid")
	lg.Info("parsing configuration json")

	if err := vmextension.UnmarshalHandlerSettings(pubJSON, protJSON, &h.publicSettings, &h.protectedSettings); err != nil {
		return h, errors.Wrap(err, "json parsing error")
	}

	lg.Info("parsed configuration json")
	lg.Info("validating configuration logically")

	if err := h.Validate(); err != nil {
		return h, errors.Wrap(err, "invalid configuration")
	}
	lg.Info("validated configuration")
	return h, nil
}

// readSettings uses specified configFolder (comes from HandlerEnvironment) to
// decrypt and parse the public/protected settings of the extension handler into
// JSON objects.
func readSettings(configFolder string) (map[string]interface{}, map[string]interface{}, error) {
	pubSettingsJSON, protSettingsJSON, err := vmextension.ReadSettings(configFolder)
	err = errors.Wrapf(err, "error reading extension configuration")
	return pubSettingsJSON, protSettingsJSON, err
}

// validateSettings takes publicSettings and protectedSettings as JSON objects
// and runs JSON schema validation on them.
func validateSettingsSchema(pubSettingsJSON, protSettingsJSON map[string]interface{}) error {
	pubJSON, err := toJSON(pubSettingsJSON)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal public settings into json")
	}
	if err := schema.ValidatePublicSettings(pubJSON); err != nil {
		return err
	}

	protJSON, err := toJSON(protSettingsJSON)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal protected settings into json")
	}
	if err := schema.ValidateProtectedSettings(protJSON); err != nil {
		return err
	}
	return nil
}

// toJSON converts given in-memory JSON object representation into a JSON object string.
func toJSON(o map[string]interface{}) (string, error) {
	if o == nil { // instead of JSON 'null' assume empty object '{}'
		return "{}", nil
	}
	b, err := json.Marshal(o)
	return string(b), errors.Wrap(err, "failed to marshal into json")
}
