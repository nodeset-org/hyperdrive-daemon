package client

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	nscommon "github.com/nodeset-org/nodeset-client-go/common"
	"github.com/rocket-pool/node-manager-core/api/client"
	"github.com/rocket-pool/node-manager-core/api/types"
)

// Requester for Constellation module calls to the nodeset.io service
type NodeSetConstellationRequester struct {
	context client.IRequesterContext
}

func NewNodeSetConstellationRequester(context client.IRequesterContext) *NodeSetConstellationRequester {
	return &NodeSetConstellationRequester{
		context: context,
	}
}

func (r *NodeSetConstellationRequester) GetName() string {
	return "NodeSet-Constellation"
}
func (r *NodeSetConstellationRequester) GetRoute() string {
	return "nodeset/constellation"
}
func (r *NodeSetConstellationRequester) GetContext() client.IRequesterContext {
	return r.context
}

// Gets the address the node's user has assigned as the registered Constellation address
func (r *NodeSetConstellationRequester) GetRegisteredAddress(deployment string) (*types.ApiResponse[api.NodeSetConstellation_GetRegisteredAddressData], error) {
	args := map[string]string{
		"deployment": deployment,
	}
	return client.SendGetRequest[api.NodeSetConstellation_GetRegisteredAddressData](r, "get-registered-address", "GetRegisteredAddress", args)
}

// Gets a signature for registering / whitelisting the node with the Constellation contracts
func (r *NodeSetConstellationRequester) GetRegistrationSignature(deployment string) (*types.ApiResponse[api.NodeSetConstellation_GetRegistrationSignatureData], error) {
	args := map[string]string{
		"deployment": deployment,
	}
	return client.SendGetRequest[api.NodeSetConstellation_GetRegistrationSignatureData](r, "get-registration-signature", "GetRegistrationSignature", args)
}

// Gets the deposit signature for a minipool from the Constellation contracts
func (r *NodeSetConstellationRequester) GetDepositSignature(deployment string, minipoolAddress common.Address, salt *big.Int) (*types.ApiResponse[api.NodeSetConstellation_GetDepositSignatureData], error) {
	args := map[string]string{
		"deployment":      deployment,
		"minipoolAddress": minipoolAddress.Hex(),
		"salt":            salt.String(),
	}
	return client.SendGetRequest[api.NodeSetConstellation_GetDepositSignatureData](r, "get-deposit-signature", "GetDepositSignature", args)
}

// Gets the validators that have been registered with the NodeSet service for this node as part of Constellation
func (r *NodeSetConstellationRequester) GetValidators(deployment string) (*types.ApiResponse[api.NodeSetConstellation_GetValidatorsData], error) {
	args := map[string]string{
		"deployment": deployment,
	}
	return client.SendGetRequest[api.NodeSetConstellation_GetValidatorsData](r, "get-validators", "GetValidators", args)
}

// Uploads signed exit messages to the NodeSet service
func (r *NodeSetConstellationRequester) UploadSignedExits(deployment string, exitMessages []nscommon.EncryptedExitData) (*types.ApiResponse[api.NodeSetConstellation_UploadSignedExitsData], error) {
	body := api.NodeSetConstellation_UploadSignedExitsRequestBody{
		Deployment:   deployment,
		ExitMessages: exitMessages,
	}
	return client.SendPostRequest[api.NodeSetConstellation_UploadSignedExitsData](r, "upload-signed-exits", "UploadSignedExits", body)
}
