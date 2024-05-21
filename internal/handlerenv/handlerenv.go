package handlerenv

import (
	"encoding/json"

	"github.com/Azure/applicationhealth-extension-linux/internal/manifest"
	"github.com/Azure/azure-extension-platform/pkg/handlerenv"
)

type HandlerEnvironment struct {
	handlerenv.HandlerEnvironment
}

func (he *HandlerEnvironment) String() string {
	env, _ := json.MarshalIndent(he, "", "\t")
	return string(env)
}

func GetHandlerEnviroment() (he *HandlerEnvironment, _ error) {
	em, err := manifest.GetExtensionManifest()
	if err != nil {
		return nil, err
	}
	env, _ := handlerenv.GetHandlerEnvironment(em.Name(), em.Version)
	return &HandlerEnvironment{
		HandlerEnvironment: *env,
	}, err
}
