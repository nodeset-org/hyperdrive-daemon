package config

import (
	"github.com/rocket-pool/node-manager-core/config"
)

const (
	// The NodeSet dev network on Holesky
	Network_HoleskyDev config.Network = "holesky-dev"

	// Local test network for development
	Network_LocalTest config.Network = "local-test"
)

// Enum to identify MEV-boost relays
type MevRelayID string

const (
	MevRelayID_Unknown            MevRelayID = ""
	MevRelayID_Flashbots          MevRelayID = "flashbots"
	MevRelayID_BloxrouteMaxProfit MevRelayID = "bloxrouteMaxProfit"
	MevRelayID_BloxrouteRegulated MevRelayID = "bloxrouteRegulated"
	MevRelayID_TitanRegional      MevRelayID = "titanRegional"
)

// Enum to describe MEV-Boost relay selection mode
type MevSelectionMode string

const (
	MevSelectionMode_All    MevSelectionMode = "all"
	MevSelectionMode_Manual MevSelectionMode = "manual"
)

// Enum to describe the transaction endpoint mode
type TxEndpointMode string

const (
	TxEndpointMode_Client           TxEndpointMode = "client"
	TxEndpointMode_FlashbotsProtect TxEndpointMode = "flashbotsProtect"
	TxEndpointMode_MevBlocker       TxEndpointMode = "mevBlocker"
	TxEndpointMode_Custom           TxEndpointMode = "custom"
)
