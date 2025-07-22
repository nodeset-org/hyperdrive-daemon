package pbs

import (
	"github.com/rocket-pool/node-manager-core/config"
	nmc_ids "github.com/rocket-pool/node-manager-core/config/ids"
)

// Constants
const (
	CommitBoostConfigFile string = "cb_config.toml"
	commitBoostPbsProdTag string = "ghcr.io/commit-boost/pbs:v0.8.0"
	commitBoostPbsTestTag string = "ghcr.io/commit-boost/pbs:v0.8.0"
)

// Configuration for Commit-Boost's PBS service
type CommitBoostPbsConfig struct {
	// The Docker Hub tag for Commit-Boost PBS
	ContainerTag config.Parameter[string]

	// Custom command line flags
	AdditionalFlags config.Parameter[string]
}

// Generates a new Commit-Boost PBS service configuration
func NewCommitBoostPbsConfig() *CommitBoostPbsConfig {
	return &CommitBoostPbsConfig{
		ContainerTag: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.ContainerTagID,
				Name:               "Container Tag",
				Description:        "The tag name of the Commit-Boost PBS container you want to use.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: true,
			},
			Default: map[config.Network]string{
				config.Network_Hoodi: commitBoostPbsTestTag,
				config.Network_All:   commitBoostPbsProdTag,
			},
		},

		AdditionalFlags: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.AdditionalFlagsID,
				Name:               "Additional Flags",
				Description:        "Additional custom command line flags you want to pass to Commit-Boost PBS, to take advantage of other settings that Hyperdrive's configuration doesn't cover.",
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
func (cfg *CommitBoostPbsConfig) GetTitle() string {
	return "Commit Boost PBS"
}

// Get the Parameters for this config
func (cfg *CommitBoostPbsConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.ContainerTag,
		&cfg.AdditionalFlags,
	}
}

// Get the sections underneath this one
func (cfg *CommitBoostPbsConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{}
}

func (cfg *CommitBoostPbsConfig) GetCommitBoostConfigFilename() string {
	return CommitBoostConfigFile
}
