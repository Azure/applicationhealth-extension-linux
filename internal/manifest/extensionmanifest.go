package manifest

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-extension-platform/pkg/utils"
)

// manifestFileName is the name of the manifest file.
const (
	manifestFileName = "manifest.xml"
)

// GetDirFunc is a function type that returns a directory path and an error.
type GetDirFunc func() (string, error)

var (
	// Set a package-level variable for the directory function
	getDir GetDirFunc = utils.GetCurrentProcessWorkingDir
)

// ExtensionManifest represents the structure of an extension manifest.
type ExtensionManifest struct {
	ProviderNameSpace   string `xml:"ProviderNameSpace"`
	Type                string `xml:"Type"`
	Version             string `xml:"Version"`
	Label               string `xml:"Label"`
	HostingResources    string `xml:"HostingResources"`
	MediaLink           string `xml:"MediaLink"`
	Description         string `xml:"Description"`
	IsInternalExtension bool   `xml:"IsInternalExtension"`
	IsJsonExtension     bool   `xml:"IsJsonExtension"`
	SupportedOS         string `xml:"SupportedOS"`
	CompanyName         string `xml:"CompanyName"`
}

// Name returns the formatted name of the extension manifest.
func (em *ExtensionManifest) Name() string {
	return fmt.Sprintf("%s.%s", em.ProviderNameSpace, em.Type)
}

// GetExtensionManifest retrieves the extension manifest from the specified directory.
// If getDir is nil, it uses the current process working directory.
// It returns the extension manifest and an error, if any.
func GetExtensionManifest() (*ExtensionManifest, error) {
	dir, err := getDir()
	if err != nil {
		return nil, err
	}

	fp, err := findManifestFilePath(dir)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	var manifest ExtensionManifest
	err = decoder.Decode(&manifest)

	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

// findManifestFilePath finds the path of the manifest file in the specified directory.
// It returns the path and an error, if any.
func findManifestFilePath(dir string) (string, error) {
	var (
		paths = []string{
			filepath.Join(dir, manifestFileName),       // this level (i.e. executable is in [EXT_NAME]/.)
			filepath.Join(dir, "..", manifestFileName), // one up (i.e. executable is in [EXT_NAME]/bin/.)
		}
	)

	for _, p := range paths {
		_, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot read file at path %s: %v", p, err)
		} else if err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("cannot find HandlerEnvironment at paths: %s", strings.Join(paths, ", "))
}
