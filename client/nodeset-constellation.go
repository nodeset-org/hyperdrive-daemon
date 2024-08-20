package client

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
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

// Gets a signature for registering / whitelisting the node with the Constellation contracts
func (r *NodeSetConstellationRequester) GetRegistrationSignature() (*types.ApiResponse[api.NodeSetConstellation_GetRegistrationSignatureData], error) {
	args := map[string]string{}
	return client.SendGetRequest[api.NodeSetConstellation_GetRegistrationSignatureData](r, "get-registration-signature", "GetRegistrationSignature", args)
}

// Gets the deposit signature for a minipool from the Constellation contracts
func (r *NodeSetConstellationRequester) GetDepositSignature(minipoolAddress common.Address, salt *big.Int) (*types.ApiResponse[api.NodeSetConstellation_GetDepositSignatureData], error) {
	args := map[string]string{
		"minipoolAddress": minipoolAddress.Hex(),
		"salt":            salt.String(),
	}
	return client.SendGetRequest[api.NodeSetConstellation_GetDepositSignatureData](r, "get-deposit-signature", "GetDepositSignature", args)
}
