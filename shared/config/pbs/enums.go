package pbs

import "github.com/rocket-pool/node-manager-core/config"

type PbsRelayID string

// Enum to identify MEV-boost relays
const (
	MevRelayID_Unknown            PbsRelayID = ""
	PbsRelayID_Flashbots          PbsRelayID = "flashbots"
	PbsRelayID_BloxrouteMaxProfit PbsRelayID = "bloxrouteMaxProfit"
	PbsRelayID_BloxrouteRegulated PbsRelayID = "bloxrouteRegulated"
	PbsRelayID_TitanRegional      PbsRelayID = "titanRegional"
)

type PbsClient string

// Enum to identify PBS clients
const (
	PbsClient_CommitBoost PbsClient = "commitBoost"
	PbsClient_MevBoost    PbsClient = "mevBoost"
)

type PbsRelaySelectionMode string

// Enum to describe PBS relay selection mode
const (
	PbsRelaySelectionMode_All    PbsRelaySelectionMode = "all"
	PbsRelaySelectionMode_Manual PbsRelaySelectionMode = "manual"
)

const (
	ContainerID_Pbs config.ContainerID = "pbs"
)
