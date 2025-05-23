package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rocket-pool/node-manager-core/config"
	"gopkg.in/yaml.v3"
)

const (
	// The nodeset.io URL for production usage
	NodesetUrlProd string = "https://nodeset.io/api"

	// The nodeset.io URL for development / staging
	NodesetUrlStaging string = "https://staging.nodeset.io/api"
)

var (
	// Mainnet resources for reference in testing
	MainnetResourcesReference *HyperdriveResources = &HyperdriveResources{
		NodeSetApiUrl: NodesetUrlProd,
	}

	// Hoodi resources for reference in testing
	HoodiResourcesReference *HyperdriveResources = &HyperdriveResources{
		NodeSetApiUrl: NodesetUrlProd,
	}

	// Hoodi Devnet resources for reference in testing
	HoodiDevResourcesReference *HyperdriveResources = &HyperdriveResources{
		NodeSetApiUrl: NodesetUrlStaging,
	}
)

// Network settings with a field for Hyperdrive-specific settings
type HyperdriveSettings struct {
	*config.NetworkSettings `yaml:",inline"`

	// Hyperdrive resources for the network
	HyperdriveResources *HyperdriveResources `yaml:"hyperdriveResources" json:"hyperdriveResources"`
}

// A collection of network-specific resources and getters for them
type HyperdriveResources struct {
	// The URL for the NodeSet API server
	NodeSetApiUrl string `yaml:"nodeSetApiUrl" json:"nodeSetApiUrl"`

	// The pubkey used to encrypt messages to nodeset.io
	EncryptionPubkey string `yaml:"encryptionPubkey" json:"encryptionPubkey"`
}

// An aggregated collection of resources for the selected network, including Hyperdrive resources
type MergedResources struct {
	// Base network resources
	*config.NetworkResources

	// Hyperdrive resources
	*HyperdriveResources
}

// Load network settings from a folder
func LoadSettingsFiles(sourceDir string) ([]*HyperdriveSettings, error) {
	// Make sure the folder exists
	_, err := os.Stat(sourceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("network settings folder [%s] does not exist", sourceDir)
	}

	// Enumerate the dir
	files, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("error enumerating override source folder: %w", err)
	}

	settingsList := []*HyperdriveSettings{}
	for _, file := range files {
		// Ignore dirs and nonstandard files
		if file.IsDir() || !file.Type().IsRegular() {
			continue
		}

		// Load the file
		filename := file.Name()
		ext := filepath.Ext(filename)
		if ext != ".yaml" && ext != ".yml" {
			// Only load YAML files
			continue
		}
		settingsFilePath := filepath.Join(sourceDir, filename)
		bytes, err := os.ReadFile(settingsFilePath)
		if err != nil {
			return nil, fmt.Errorf("error reading network settings file [%s]: %w", settingsFilePath, err)
		}

		// Unmarshal the settings
		settings := new(HyperdriveSettings)
		err = yaml.Unmarshal(bytes, settings)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling network settings file [%s]: %w", settingsFilePath, err)
		}
		settingsList = append(settingsList, settings)
	}
	return settingsList, nil
}
