package pbs

import (
	"github.com/rocket-pool/node-manager-core/config"
	nmc_ids "github.com/rocket-pool/node-manager-core/config/ids"
)

// Constants
const (
	mevBoostProdTag string = "flashbots/mev-boost:1.9"
	mevBoostTestTag string = "flashbots/mev-boost:1.9"
)

// Configuration for MEV-Boost
type MevBoostConfig struct {
	// The Docker Hub tag for MEV-Boost
	ContainerTag config.Parameter[string]

	// Custom command line flags
	AdditionalFlags config.Parameter[string]
}

// Generates a new MEV-Boost configuration
func NewMevBoostConfig() *MevBoostConfig {
	return &MevBoostConfig{
		ContainerTag: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.ContainerTagID,
				Name:               "Container Tag",
				Description:        "The tag name of the MEV-Boost container you want to use on Docker Hub.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: true,
			},
			Default: map[config.Network]string{
				config.Network_Hoodi: mevBoostTestTag,
				config.Network_All:   mevBoostProdTag,
			},
		},

		AdditionalFlags: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.AdditionalFlagsID,
				Name:               "Additional Flags",
				Description:        "Additional custom command line flags you want to pass to MEV-Boost, to take advantage of other settings that Hyperdrive's configuration doesn't cover.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
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
func (cfg *MevBoostConfig) GetTitle() string {
	return "MEV-Boost"
}

// Get the Parameters for this config
func (cfg *MevBoostConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.ContainerTag,
		&cfg.AdditionalFlags,
	}
}

// Get the sections underneath this one
func (cfg *MevBoostConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{}
}
