package version

import (
	"encoding/xml"
	"os"
)

const (
	ExtensionManifestFileName = "manifest.xml"
)

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

func getExtensionManifest(filepath string) (ExtensionManifest, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return ExtensionManifest{}, err
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	var manifest ExtensionManifest
	err = decoder.Decode(&manifest)

	if err != nil {
		return ExtensionManifest{}, err
	}
	return manifest, nil
}
