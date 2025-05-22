package ns_stakewise

import (
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/gorilla/mux"
	hdcommon "github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	apiv0 "github.com/nodeset-org/nodeset-client-go/api-v0"
	v3stakewise "github.com/nodeset-org/nodeset-client-go/api-v3/stakewise"
	"github.com/nodeset-org/nodeset-client-go/common/stakewise"

	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/api/types"
)

// ===============
// === Factory ===
// ===============

type stakeWiseGetValidatorManagerSignatureContextFactory struct {
	handler *StakeWiseHandler
}

func (f *stakeWiseGetValidatorManagerSignatureContextFactory) Create(body api.NodeSetStakeWise_GetValidatorManagerSignatureRequestBody) (*stakeWiseGetValidatorManagerSignatureContext, error) {
	c := &stakeWiseGetValidatorManagerSignatureContext{
		handler: f.handler,
		body:    body,
	}
	return c, nil
}

func (f *stakeWiseGetValidatorManagerSignatureContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterQuerylessPost[*stakeWiseGetValidatorManagerSignatureContext, api.NodeSetStakeWise_GetValidatorManagerSignatureRequestBody, api.NodeSetStakeWise_GetValidatorManagerSignatureData](
		router, "get-validator-manager-signature", f, f.handler.logger.Logger, f.handler.serviceProvider,
	)
}

// ===============
// === Context ===
// ===============

type stakeWiseGetValidatorManagerSignatureContext struct {
	handler *StakeWiseHandler
	body    api.NodeSetStakeWise_GetValidatorManagerSignatureRequestBody
}

func (c *stakeWiseGetValidatorManagerSignatureContext) PrepareData(data *api.NodeSetStakeWise_GetValidatorManagerSignatureData, opts *bind.TransactOpts) (types.ResponseStatus, error) {
	sp := c.handler.serviceProvider
	ctx := c.handler.ctx

	// Requirements
	err := sp.RequireWalletReady()
	if err != nil {
		return types.ResponseStatus_WalletNotReady, err
	}
	err = sp.RequireRegisteredWithNodeSet(ctx)
	if err != nil {
		if errors.Is(err, hdcommon.ErrNotRegisteredWithNodeSet) {
			data.NotRegistered = true
			return types.ResponseStatus_Success, nil
		}
		return types.ResponseStatus_Error, err
	}

	// Request the signature
	ns := sp.GetNodeSetServiceManager()
	signature, err := ns.StakeWise_GetValidatorManagerSignature(
		ctx,
		c.body.Deployment,
		c.body.Vault,
		c.body.BeaconDepositRoot,
		c.body.DepositData,
		c.body.EncryptedExitMessages,
	)
	if err != nil {
		if errors.Is(err, apiv0.ErrVaultNotFound) {
			data.VaultNotFound = true
			return types.ResponseStatus_Success, nil
		}
		if errors.Is(err, stakewise.ErrInvalidPermissions) {
			data.InvalidPermissions = true
			return types.ResponseStatus_Success, nil
		}
		if errors.Is(err, v3stakewise.ErrDepositRootAlreadyAssigned) {
			data.DepositRootAlreadyUsed = true
			return types.ResponseStatus_Success, nil
		}
		return types.ResponseStatus_Error, err
	}

	// Success
	data.Signature = signature
	return types.ResponseStatus_Success, nil
}
