package pbs

import (
	"fmt"
	"strings"

	"github.com/nodeset-org/hyperdrive-daemon/shared/config/ids"
	"github.com/rocket-pool/node-manager-core/config"
	nmc_ids "github.com/rocket-pool/node-manager-core/config/ids"
)

// A MEV relay
type PbsRelay struct {
	ID          PbsRelayID
	Name        string
	Description string
	Urls        map[string]string
}

// Configuration for locally managed PBS services
type LocalPbsClientConfig struct {
	// The PBS client to use
	Client config.Parameter[PbsClient]

	// The mode for relay selection
	RelaySelectionMode config.Parameter[PbsRelaySelectionMode]

	// Flashbots relay
	FlashbotsRelay config.Parameter[bool]

	// bloXroute max profit relay
	BloxRouteMaxProfitRelay config.Parameter[bool]

	// bloXroute regulated relay
	BloxRouteRegulatedRelay config.Parameter[bool]

	// Titan regional relay
	TitanRegionalRelay config.Parameter[bool]

	// Custom relays provided by the user
	CustomRelays config.Parameter[string]

	// The RPC port
	Port config.Parameter[uint16]

	// Toggle for forwarding the HTTP port outside of Docker
	OpenRpcPort config.Parameter[config.RpcPortMode]

	CommitBoostPbsConfig *CommitBoostPbsConfig

	MevBoostConfig *MevBoostConfig

	///////////////////////////
	// Non-editable settings //
	///////////////////////////

	relays   []PbsRelay
	relayMap map[PbsRelayID]PbsRelay
}

// Generates a new local PBS service configuration
func NewLocalPbsServiceConfig() *LocalPbsClientConfig {
	// Generate the relays
	relays := createDefaultRelays()
	relayMap := map[PbsRelayID]PbsRelay{}
	for _, relay := range relays {
		relayMap[relay.ID] = relay
	}

	rpcPortModes := config.GetPortModes("")

	return &LocalPbsClientConfig{
		Client: config.Parameter[PbsClient]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsClientID,
				Name:               "PBS Client",
				Description:        "Select the PBS client to use.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: []*config.ParameterOption[PbsClient]{{
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "Commit-Boost",
					Description: "Commit-Boost is a powerful PBS client that offers detailed reporting and transparency around block building, very low latency, the ability to specify relays on a per-validator basis, and a responsive development team with a proven track record of quickly addressing feedback. It is fully open source and written in Rust. Visit https://github.com/commit-boost/commit-boost-client/ to learn more.",
				},
				Value: PbsClient_CommitBoost,
			}, {
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "MEV-Boost",
					Description: "MEV-Boost is the oldest public implementation of a client for the PBS marketplace. It is developed by Flashbots and is completely open source and written in Go. Visit https://github.com/flashbots/mev-boost to learn more.",
				},
				Value: PbsClient_MevBoost,
			}},
			Default: map[config.Network]PbsClient{
				config.Network_All: PbsClient_CommitBoost,
			},
		},

		RelaySelectionMode: config.Parameter[PbsRelaySelectionMode]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsRelaySelectionModeID,
				Name:               "Selection Mode",
				Description:        "Select how the TUI shows you the options for which PBS relays to enable.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: []*config.ParameterOption[PbsRelaySelectionMode]{{
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "Use All Relays",
					Description: "Use this if you simply want to enable all of the built-in relays without needing to read about each individual relay. If new relays get added to Hyperdrive, you'll automatically start using those too.\n\nNote that all of Hyperdrive's built-in relays support regional sanction lists (such as the US OFAC list) and are compliant with regulations. To learn more, please visit https://medium.com/coinmonks/understanding-the-impact-of-the-ofac-sanctions-on-block-builders-9c0e02b7e450.",
				},
				Value: PbsRelaySelectionMode_All,
			}, {
				ParameterOptionCommon: &config.ParameterOptionCommon{
					Name:        "Manual Mode",
					Description: "Each relay will be shown, and you can enable each one individually as you see fit.\nUse this if you already know about the relays and want to customize the ones you will use.\n\nNote that all of Hyperdrive's built-in relays support regional sanction lists (such as the US OFAC list) and are compliant with regulations. To learn more, please visit https://medium.com/coinmonks/understanding-the-impact-of-the-ofac-sanctions-on-block-builders-9c0e02b7e450.",
				},
				Value: PbsRelaySelectionMode_Manual,
			}},
			Default: map[config.Network]PbsRelaySelectionMode{
				config.Network_All: PbsRelaySelectionMode_All,
			},
		},

		// Explicit relay params
		FlashbotsRelay:          generateRelayParameter(ids.PbsFlashbotsID, relayMap[PbsRelayID_Flashbots]),
		BloxRouteMaxProfitRelay: generateRelayParameter(ids.PbsBloxRouteMaxProfitID, relayMap[PbsRelayID_BloxrouteMaxProfit]),
		BloxRouteRegulatedRelay: generateRelayParameter(ids.PbsBloxRouteRegulatedID, relayMap[PbsRelayID_BloxrouteRegulated]),
		TitanRegionalRelay:      generateRelayParameter(ids.PbsTitanRegionalID, relayMap[PbsRelayID_TitanRegional]),

		CustomRelays: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.PbsCustomRelaysID,
				Name:               "Custom Relays",
				Description:        "Add custom relay URLs to the PBS client that aren't part of the built-in set. You can add multiple relays by separating each one with a comma. Any relay URLs can be used as long as they match your selected Ethereum network.\n\nFor a comprehensive list of available relays, we recommend the list maintained by ETHStaker:\nhttps://github.com/eth-educators/ethstaker-guides/blob/main/MEV-relay-list.md",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         true,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]string{
				config.Network_All: "",
			},
		},

		Port: config.Parameter[uint16]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.PortID,
				Name:               "Port",
				Description:        "The port that the PBS client should serve its API on.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]uint16{
				config.Network_All: uint16(18550),
			},
		},

		OpenRpcPort: config.Parameter[config.RpcPortMode]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 nmc_ids.OpenPortID,
				Name:               "Expose API Port",
				Description:        "Expose the API port to other processes on your machine, or to your local network so other local machines can access the PBS client's API.",
				AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: rpcPortModes,
			Default: map[config.Network]config.RpcPortMode{
				config.Network_All: config.RpcPortMode_Closed,
			},
		},

		CommitBoostPbsConfig: NewCommitBoostPbsConfig(),
		MevBoostConfig:       NewMevBoostConfig(),

		relays:   relays,
		relayMap: relayMap,
	}
}

// The title for the config
func (cfg *LocalPbsClientConfig) GetTitle() string {
	return "Local PBS Service"
}

// Get the Parameters for this config
func (cfg *LocalPbsClientConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.Client,
		&cfg.RelaySelectionMode,
		&cfg.FlashbotsRelay,
		&cfg.BloxRouteMaxProfitRelay,
		&cfg.BloxRouteRegulatedRelay,
		&cfg.TitanRegionalRelay,
		&cfg.CustomRelays,
		&cfg.Port,
		&cfg.OpenRpcPort,
	}
}

// Get the sections underneath this one
func (cfg *LocalPbsClientConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{
		ids.CommitBoostPbsID: cfg.CommitBoostPbsConfig,
		ids.MevBoostPbsID:    cfg.MevBoostConfig,
	}
}

// Checks if any relays are available for the current network
func (cfg *LocalPbsClientConfig) HasRelays(networkName string) bool {
	if networkName == "" {
		return false
	}

	// Check if any of the relays are available for that Eth network
	for _, relay := range cfg.relays {
		_, exists := relay.Urls[networkName]
		if !exists {
			continue
		}
		return true
	}

	return false
}

// Get the relays that are available for the current network
func (cfg *LocalPbsClientConfig) GetAvailableRelays(networkName string) []PbsRelay {
	relays := []PbsRelay{}
	if networkName == "" {
		return relays
	}

	for _, relay := range cfg.relays {
		_, exists := relay.Urls[networkName]
		if !exists {
			continue
		}
		relays = append(relays, relay)
	}
	return relays
}

// Get which PBS relays are enabled
func (cfg *LocalPbsClientConfig) GetEnabledPbsRelays(networkName string) []PbsRelay {
	relays := []PbsRelay{}
	if networkName == "" {
		return relays
	}

	switch cfg.RelaySelectionMode.Value {
	case PbsRelaySelectionMode_All:
		for _, relay := range cfg.relays {
			_, exists := relay.Urls[networkName]
			if !exists {
				// Skip relays that don't exist on the current network
				continue
			}
			relays = append(relays, relay)
		}

	case PbsRelaySelectionMode_Manual:
		if cfg.FlashbotsRelay.Value {
			_, exists := cfg.relayMap[PbsRelayID_Flashbots].Urls[networkName]
			if exists {
				relays = append(relays, cfg.relayMap[PbsRelayID_Flashbots])
			}
		}
		if cfg.BloxRouteMaxProfitRelay.Value {
			_, exists := cfg.relayMap[PbsRelayID_BloxrouteMaxProfit].Urls[networkName]
			if exists {
				relays = append(relays, cfg.relayMap[PbsRelayID_BloxrouteMaxProfit])
			}
		}
		if cfg.BloxRouteRegulatedRelay.Value {
			_, exists := cfg.relayMap[PbsRelayID_BloxrouteRegulated].Urls[networkName]
			if exists {
				relays = append(relays, cfg.relayMap[PbsRelayID_BloxrouteRegulated])
			}
		}
		if cfg.TitanRegionalRelay.Value {
			_, exists := cfg.relayMap[PbsRelayID_TitanRegional].Urls[networkName]
			if exists {
				relays = append(relays, cfg.relayMap[PbsRelayID_TitanRegional])
			}
		}
	}

	return relays
}

func (cfg *LocalPbsClientConfig) GetRelayString(networkName string) string {
	relayUrls := []string{}
	if networkName == "" {
		return ""
	}

	relays := cfg.GetEnabledPbsRelays(networkName)
	for _, relay := range relays {
		relayUrls = append(relayUrls, relay.Urls[networkName])
	}
	if cfg.CustomRelays.Value != "" {
		relayUrls = append(relayUrls, cfg.CustomRelays.Value)
	}

	relayString := strings.Join(relayUrls, ",")
	return relayString
}

// Create the default PBS relays
func createDefaultRelays() []PbsRelay {
	relays := []PbsRelay{
		// Flashbots
		{
			ID:          PbsRelayID_Flashbots,
			Name:        "Flashbots",
			Description: "Flashbots is the developer of MEV-Boost, and one of the best-known and most trusted relays in the space.",
			Urls: map[string]string{
				config.EthNetwork_Mainnet: "https://0xac6e77dfe25ecd6110b8e780608cce0dab71fdd5ebea22a16c0205200f2f8e2e3ad3b71d3499c54ad14d6c21b41a37ae@boost-relay.flashbots.net?id=hyperdrive",
				config.EthNetwork_Hoodi:   "https://0xafa4c6985aa049fb79dd37010438cfebeb0f2bd42b115b89dd678dab0670c1de38da0c4e9138c9290a398ecd9a0b3110@boost-relay-hoodi.flashbots.net",
			},
		},

		// bloXroute Max Profit
		{
			ID:          PbsRelayID_BloxrouteMaxProfit,
			Name:        "bloXroute Max Profit",
			Description: "Select this to enable the \"max profit\" relay from bloXroute.",
			Urls: map[string]string{
				config.EthNetwork_Mainnet: "https://0x8b5d2e73e2a3a55c6c87b8b6eb92e0149a125c852751db1422fa951e42a09b82c142c3ea98d0d9930b056a3bc9896b8f@bloxroute.max-profit.blxrbdn.com?id=hyperdrive",
				config.EthNetwork_Hoodi:   "https://0x821f2a65afb70e7f2e820a925a9b4c80a159620582c1766b1b09729fec178b11ea22abb3a51f07b288be815a1a2ff516@bloxroute.hoodi.blxrbdn.com",
			},
		},

		// bloXroute Regulated
		{
			ID:          PbsRelayID_BloxrouteRegulated,
			Name:        "bloXroute Regulated",
			Description: "Select this to enable the \"regulated\" relay from bloXroute.",
			Urls: map[string]string{
				config.EthNetwork_Mainnet: "https://0xb0b07cd0abef743db4260b0ed50619cf6ad4d82064cb4fbec9d3ec530f7c5e6793d9f286c4e082c0244ffb9f2658fe88@bloxroute.regulated.blxrbdn.com?id=hyperdrive",
			},
		},

		// Titan Regional
		{
			ID:          PbsRelayID_TitanRegional,
			Name:        "Titan Regional",
			Description: "Titan Relay is a neutral, Rust-based PBS Relay optimized for low latency through put, geographical distribution, and robustness. This is the regulated (censoring) version.",
			Urls: map[string]string{
				config.EthNetwork_Mainnet: "https://0x8c4ed5e24fe5c6ae21018437bde147693f68cda427cd1122cf20819c30eda7ed74f72dece09bb313f2a1855595ab677d@regional.titanrelay.xyz",
				config.EthNetwork_Hoodi:   "https://0xaa58208899c6105603b74396734a6263cc7d947f444f396a90f7b7d3e65d102aec7e5e5291b27e08d02c50a050825c2f@hoodi.titanrelay.xyz",
			},
		},
	}

	return relays
}

// Generate one of the relay parameters
func generateRelayParameter(id string, relay PbsRelay) config.Parameter[bool] {
	description := fmt.Sprintf("[lime]NOTE: You can enable multiple options.\n\n[white]%s\n\n", relay.Description)

	return config.Parameter[bool]{
		ParameterCommon: &config.ParameterCommon{
			ID:                 id,
			Name:               fmt.Sprintf("Enable %s", relay.Name),
			Description:        description,
			AffectsContainers:  []config.ContainerID{ContainerID_Pbs},
			CanBeBlank:         false,
			OverwriteOnUpgrade: false,
		},
		Default: map[config.Network]bool{
			config.Network_All: false,
		},
	}
}

// Get the container tag for the currently selected PBS client
func (cfg *LocalPbsClientConfig) GetContainerTag() string {
	switch cfg.Client.Value {
	case PbsClient_CommitBoost:
		return cfg.CommitBoostPbsConfig.ContainerTag.Value
	case PbsClient_MevBoost:
		return cfg.MevBoostConfig.ContainerTag.Value
	default:
		return ""
	}
}

// Get the additional flags for the currently selected PBS client
func (cfg *LocalPbsClientConfig) GetAdditionalFlags() string {
	switch cfg.Client.Value {
	case PbsClient_CommitBoost:
		return cfg.CommitBoostPbsConfig.AdditionalFlags.Value
	case PbsClient_MevBoost:
		return cfg.MevBoostConfig.AdditionalFlags.Value
	default:
		return ""
	}
}

// Used by text/template to format pbs.yml
func (cfg *LocalPbsClientConfig) GetOpenPorts() string {
	portMode := cfg.OpenRpcPort.Value
	if !portMode.IsOpen() {
		return ""
	}
	port := cfg.Port.Value
	return fmt.Sprintf("\"%s\"", portMode.DockerPortMapping(port))
}
