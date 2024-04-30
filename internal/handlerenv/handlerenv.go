package handlerenv

import (
	"fmt"

	"github.com/Azure/applicationhealth-extension-linux/internal/manifest"
	"github.com/Azure/azure-extension-platform/pkg/handlerenv"
)

type HandlerEnvironment struct {
	*handlerenv.HandlerEnvironment
}

func (he HandlerEnvironment) String() string {
	return fmt.Sprintf(
		"HandlerEnvironment{HeartbeatFile: %s, StatusFolder: %s, ConfigFolder: %s, LogFolder: %s, DataFolder: %s, EventsFolder: %s, DeploymentID: %s, RoleName: %s, Instance: %s, HostResolverAddress: %s}",
		he.HeartbeatFile,
		he.StatusFolder,
		he.ConfigFolder,
		he.LogFolder,
		he.DataFolder,
		he.EventsFolder,
		he.DeploymentID,
		he.RoleName,
		he.Instance,
		he.HostResolverAddress)
}

func GetHandlerEnviroment() (he *HandlerEnvironment, _ error) {
	em, err := manifest.GetExtensionManifest()
	if err != nil {
		return nil, err
	}
	env, _ := handlerenv.GetHandlerEnvironment(em.Name(), em.Version)
	return &HandlerEnvironment{
		env,
	}, err
}
