package pbs

import (
	ids "github.com/nodeset-org/hyperdrive-daemon/shared/config/ids"
	"github.com/rocket-pool/node-manager-core/config"
)

// Configuration for locally managed PBS services
type ExternalPbsClientConfig struct {
	// The URL of an external MEV-Boost client
	ExternalUrl config.Parameter[string]
}

// Generates a new external PBS service configuration
func NewExternalPbsServiceConfig() *ExternalPbsClientConfig {
	return &ExternalPbsClientConfig{
		ExternalUrl: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsExternalUrlID,
				Name:               "External URL",
				Description:        "The URL of the external PBS service.\nNOTE: If you are running it on the same machine as this node, addresses like `localhost` and `127.0.0.1` will not work due to Docker limitations. Enter your machine's LAN IP address instead, for example 'http://192.168.1.100:18550'.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode},
				CanBeBlank:         true,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]string{
				config.Network_All: "",
			},
		},
	}
}

// The title for the config
func (cfg *ExternalPbsClientConfig) GetTitle() string {
	return "External PBS Service"
}

// Get the Parameters for this config
func (cfg *ExternalPbsClientConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.ExternalUrl,
	}
}

// Get the sections underneath this one
func (cfg *ExternalPbsClientConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{}
}
