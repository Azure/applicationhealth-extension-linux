package handlerenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-extension-platform/pkg/handlerenv"
	"github.com/Azure/azure-extension-platform/pkg/utils"
)

// HandlerEnvFileName is the file name of the Handler Environment as placed by the
// Azure Linux Guest Agent.
const HandlerEnvFileName = "HandlerEnvironment.json"

type HandlerEnvironment = handlerenv.HandlerEnvironment

// HandlerEnvironment describes the handler environment configuration presented
// to the extension handler by the Azure Linux Guest Agent.
type handlerEnvironmentInternal struct {
	Version            float64 `json:"version"`
	Name               string  `json:"name"`
	HandlerEnvironment struct {
		HeartbeatFile       string `json:"heartbeatFile"`
		StatusFolder        string `json:"statusFolder"`
		ConfigFolder        string `json:"configFolder"`
		LogFolder           string `json:"logFolder"`
		EventsFolder        string `json:"eventsFolder"`
		DeploymentID        string `json:"deploymentid"`
		RoleName            string `json:"rolename"`
		Instance            string `json:"instance"`
		HostResolverAddress string `json:"hostResolverAddress"`
	}
}

// GetHandlerEnv locates the HandlerEnvironment.json file by assuming it lives
// next to or one level above the extension handler (read: this) executable,
// reads, parses and returns it.
func getHandlerEnviromentInternal() (he handlerEnvironmentInternal, _ error) {
	dir, err := utils.GetCurrentProcessWorkingDir()
	if err != nil {
		return he, fmt.Errorf("vmextension: cannot find base directory of the running process: %v", err)
	}
	paths := []string{
		filepath.Join(dir, HandlerEnvFileName),       // this level (i.e. executable is in [EXT_NAME]/.)
		filepath.Join(dir, "..", HandlerEnvFileName), // one up (i.e. executable is in [EXT_NAME]/bin/.)
	}
	var b []byte
	for _, p := range paths {
		o, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return he, fmt.Errorf("vmextension: error examining HandlerEnvironment at '%s': %v", p, err)
		} else if err == nil {
			b = o
			break
		}
	}
	if b == nil {
		return he, fmt.Errorf("vmextension: Cannot find HandlerEnvironment at paths: %s", strings.Join(paths, ", "))
	}
	return parseHandlerEnv(b)
}

// parseHandlerEnv parses the
// /var/lib/waagent/[extension]/HandlerEnvironment.json format.
func parseHandlerEnv(b []byte) (he handlerEnvironmentInternal, _ error) {
	var hf []handlerEnvironmentInternal

	if err := json.Unmarshal(b, &hf); err != nil {
		return he, fmt.Errorf("vmextension: failed to parse handler env: %v", err)
	}
	if len(hf) != 1 {
		return he, fmt.Errorf("vmextension: expected 1 config in parsed HandlerEnvironment, found: %v", len(hf))
	}
	return hf[0], nil
}

func GetHandlerEnviroment() (he *HandlerEnvironment, _ error) {
	h, err := getHandlerEnviromentInternal()
	if err != nil {
		return nil, err
	}
	dataFolder := utils.GetDataFolder(h.Name, fmt.Sprintf("%v", h.Version))
	return &HandlerEnvironment{
		HeartbeatFile:       h.HandlerEnvironment.HeartbeatFile,
		StatusFolder:        h.HandlerEnvironment.StatusFolder,
		ConfigFolder:        h.HandlerEnvironment.ConfigFolder,
		LogFolder:           h.HandlerEnvironment.LogFolder,
		DataFolder:          dataFolder,
		EventsFolder:        h.HandlerEnvironment.EventsFolder,
		DeploymentID:        h.HandlerEnvironment.DeploymentID,
		RoleName:            h.HandlerEnvironment.RoleName,
		Instance:            h.HandlerEnvironment.Instance,
		HostResolverAddress: h.HandlerEnvironment.HostResolverAddress,
	}, nil
}
