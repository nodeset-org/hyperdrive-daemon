package client

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	"github.com/rocket-pool/node-manager-core/api/client"
	"github.com/rocket-pool/node-manager-core/api/types"
	"github.com/rocket-pool/node-manager-core/beacon"
)

// Requester for StakeWise module calls to the nodeset.io service
type NodeSetStakeWiseRequester struct {
	context client.IRequesterContext
}

func NewNodeSetStakeWiseRequester(context client.IRequesterContext) *NodeSetStakeWiseRequester {
	return &NodeSetStakeWiseRequester{
		context: context,
	}
}

func (r *NodeSetStakeWiseRequester) GetName() string {
	return "NodeSet-StakeWise"
}
func (r *NodeSetStakeWiseRequester) GetRoute() string {
	return "nodeset/stakewise"
}
func (r *NodeSetStakeWiseRequester) GetContext() client.IRequesterContext {
	return r.context
}

// Gets the list of vaults on the given deployment
func (r *NodeSetStakeWiseRequester) GetVaults(deployment string) (*types.ApiResponse[api.NodeSetStakeWise_GetVaultsData], error) {
	args := map[string]string{
		"deployment": deployment,
	}
	return client.SendGetRequest[api.NodeSetStakeWise_GetVaultsData](r, "get-vaults", "GetVaults", args)
}

// Gets the list of validators that the node has registered with the provided vault
func (r *NodeSetStakeWiseRequester) GetRegisteredValidators(deployment string, vault common.Address) (*types.ApiResponse[api.NodeSetStakeWise_GetRegisteredValidatorsData], error) {
	args := map[string]string{
		"deployment": deployment,
		"vault":      vault.Hex(),
	}
	return client.SendGetRequest[api.NodeSetStakeWise_GetRegisteredValidatorsData](r, "get-registered-validators", "GetRegisteredValidators", args)
}

// Gets info about the number of validators the node account has, and how many more it can register
func (r *NodeSetStakeWiseRequester) GetValidatorsInfo(deployment string, vault common.Address) (*types.ApiResponse[api.NodeSetStakeWise_GetValidatorsInfoData], error) {
	args := map[string]string{
		"deployment": deployment,
		"vault":      vault.Hex(),
	}
	return client.SendGetRequest[api.NodeSetStakeWise_GetValidatorsInfoData](r, "get-validators-info", "GetValidatorsInfo", args)
}

// Uploads new validator information to NodeSet and requests a signature
func (r *NodeSetStakeWiseRequester) GetValidatorManagerSignature(deployment string, vault common.Address, beaconDepositRoot common.Hash, depositData []beacon.ExtendedDepositData, encryptedExitMessage []string) (*types.ApiResponse[api.NodeSetStakeWise_GetValidatorManagerSignatureData], error) {
	body := api.NodeSetStakeWise_GetValidatorManagerSignatureRequestBody{
		Deployment:            deployment,
		Vault:                 vault,
		BeaconDepositRoot:     beaconDepositRoot,
		DepositData:           depositData,
		EncryptedExitMessages: encryptedExitMessage,
	}
	return client.SendPostRequest[api.NodeSetStakeWise_GetValidatorManagerSignatureData](r, "get-validator-manager-signature", "GetValidatorManagerSignature", body)
}
