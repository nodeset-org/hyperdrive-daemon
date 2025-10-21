package pbs

import (
	ids "github.com/nodeset-org/hyperdrive-daemon/shared/config/ids"
	"github.com/rocket-pool/node-manager-core/config"
)

// Configuration for PBS clients
type PbsConfig struct {
	// Toggle to enable / disable
	Enable config.Parameter[bool]

	// Ownership mode
	Mode config.Parameter[config.ClientMode]

	// Local PBS client configuration
	LocalPbsClientConfig *LocalPbsClientConfig

	// External PBS client configuration
	ExternalPbsClientConfig *ExternalPbsClientConfig
}

// Generates a new PBS configuration
func NewPbsConfig() *PbsConfig {
	return &PbsConfig{
		Enable: config.Parameter[bool]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsEnableID,
				Name:               "Enable PBS Client",
				Description:        "Enable support for a PBS client. When one of your validators needs to propose a Beacon chain block, this client lets other professional services build it for you instead of building your own. These block builders find and extract extra MEV opportunities, giving you a healthy tip in return (which tends to be worth more than blocks you built on your own).",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, ContainerID_Pbs, config.ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]bool{
				config.Network_All: true,
			},
		},

		Mode: config.Parameter[config.ClientMode]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsModeID,
				Name:               "PBS Client Mode",
				Description:        "Choose whether to let Hyperdrive manage your PBS client (Locally Managed), or if you manage your own outside of Hyperdrive (Externally Managed).",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: []*config.ParameterOption[config.ClientMode]{{
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "Locally Managed",
					Description: "Allow Hyperdrive to manage a PBS client for you",
				},
				Value: config.ClientMode_Local,
			}, {
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "Externally Managed",
					Description: "Use an existing PBS client that you manage on your own",
				},
				Value: config.ClientMode_External,
			}},
			Default: map[config.Network]config.ClientMode{
				config.Network_All: config.ClientMode_Local,
			},
		},

		LocalPbsClientConfig:    NewLocalPbsServiceConfig(),
		ExternalPbsClientConfig: NewExternalPbsServiceConfig(),
	}
}

// The title for the config
func (cfg *PbsConfig) GetTitle() string {
	return "PBS Configuration"
}

// Get the Parameters for this config
func (cfg *PbsConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.Enable,
		&cfg.Mode,
	}
}

// Get the sections underneath this one
func (cfg *PbsConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{
		ids.PbsLocalID:    cfg.LocalPbsClientConfig,
		ids.PbsExternalID: cfg.ExternalPbsClientConfig,
	}
}
