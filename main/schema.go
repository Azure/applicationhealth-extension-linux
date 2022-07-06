package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

// Refer to http://json-schema.org/ on how to use JSON Schemas.

const (
	publicSettingsSchema = `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Application Health - Public Settings",
  "type": "object",
  "properties": {
    "protocol": {
      "description": "Required - can be 'tcp', 'http', or 'https'.",
      "type": "string",
      "enum": ["tcp", "http", "https"]
    },
	"port": {
	  "description": "Required when the protocol is 'tcp'. Optional when the protocol is 'http' or 'https'.",
      "type": "integer",
      "minimum": 1,
      "maximum": 65535
	},
    "requestPath": {
      "description": "Path on which the web request should be sent. Required when the protocol is 'http' or 'https'.",
      "type": "string"
    },
    "intervalInSeconds": {
      "description": "The interval, in seconds, for how frequently to probe the endpoint for health status.",
      "type": "integer",
      "default": 5,
      "minimum": 5,
      "maximum": 60
    },
    "numberOfProbes": {
      "description": "The number of probe reponses needed to change health state",
      "type": "integer",
      "default": 1,
      "minimum": 1,
      "maximum": 24
    },
    "gracePeriod": {
      "description": "The amount of time in seconds the application will default to 'Initializing' state if no valid health state is observed numberOfProbes consecutive times.",
      "type": "integer",
      "minimum": 5,
      "maximum": 3600
    }
  },
  "additionalProperties": false
}`

	protectedSettingsSchema = `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Application Health - Protected Settings",
  "type": "object",
  "properties": {
  },
  "additionalProperties": false
}`
)

// validateObjectJSON validates the specified json with schemaJSON.
// If json is empty string, it will be converted into an empty JSON object
// before being validated.
func validateObjectJSON(schema *gojsonschema.Schema, json string) error {
	if json == "" {
		json = "{}"
	}

	doc := gojsonschema.NewStringLoader(json)
	res, err := schema.Validate(doc)
	if err != nil {
		return err
	}
	if !res.Valid() {
		for _, err := range res.Errors() {
			// return with the first error
			return fmt.Errorf("%s", err)
		}
	}
	return nil
}

func validateSettingsObject(settingsType, schemaJSON, docJSON string) error {
	schema, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(schemaJSON))
	if err != nil {
		return errors.Wrapf(err, "failed to load %s settings schema", settingsType)
	}
	if err := validateObjectJSON(schema, docJSON); err != nil {
		return errors.Wrapf(err, "invalid %s settings JSON", settingsType)
	}
	return nil
}

func validatePublicSettings(json string) error {
	return validateSettingsObject("public", publicSettingsSchema, json)
}

func validateProtectedSettings(json string) error {
	return validateSettingsObject("protected", protectedSettingsSchema, json)
}
