package api

import (
	"github.com/ethereum/go-ethereum/common"
	v3stakewise "github.com/nodeset-org/nodeset-client-go/api-v3/stakewise"
	"github.com/rocket-pool/node-manager-core/beacon"
)

type NodeSetStakeWise_GetVaultsData struct {
	NotRegistered      bool                    `json:"notRegistered"`
	InvalidPermissions bool                    `json:"invalidPermissions"`
	Vaults             []v3stakewise.VaultInfo `json:"vaults"`
}

type NodeSetStakeWise_GetRegisteredValidatorsData struct {
	NotRegistered      bool                          `json:"notRegistered"`
	InvalidPermissions bool                          `json:"invalidPermissions"`
	Validators         []v3stakewise.ValidatorStatus `json:"validators"`
}

type NodeSetStakeWise_GetValidatorsInfoData struct {
	NotRegistered        bool `json:"notRegistered"`
	RegisteredValidators int  `json:"registeredValidators"`
	MaxValidators        int  `json:"maxValidators"`
	AvailableValidators  int  `json:"availableValidators"`
}

type NodeSetStakeWise_GetValidatorManagerSignatureRequestBody struct {
	Deployment            string                       `json:"deployment"`
	Vault                 common.Address               `json:"vault"`
	BeaconDepositRoot     common.Hash                  `json:"beaconDepositRoot"`
	DepositData           []beacon.ExtendedDepositData `json:"depositData"`
	EncryptedExitMessages []string                     `json:"encryptedExitMessages"`
}

type NodeSetStakeWise_GetValidatorManagerSignatureData struct {
	NotRegistered          bool   `json:"notRegistered"`
	VaultNotFound          bool   `json:"vaultNotFound"`
	InvalidPermissions     bool   `json:"invalidPermissions"`
	DepositRootAlreadyUsed bool   `json:"depositRootAlreadyUsed"`
	Signature              string `json:"signature"`
}
