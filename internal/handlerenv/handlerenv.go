package handlerenv

import (
	"github.com/Azure/applicationhealth-extension-linux/internal/manifest"
	"github.com/Azure/azure-extension-platform/pkg/handlerenv"
)

type HandlerEnvironment = handlerenv.HandlerEnvironment

func GetHandlerEnviroment() (he *HandlerEnvironment, _ error) {
	em, err := manifest.GetExtensionManifest()
	if err != nil {
		return nil, err
	}
	return handlerenv.GetHandlerEnvironment(em.Name(), em.Version)
}
