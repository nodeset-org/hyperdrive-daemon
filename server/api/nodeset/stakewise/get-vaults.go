package ns_stakewise

import (
	"errors"
	"net/url"

	hdcommon "github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	"github.com/nodeset-org/nodeset-client-go/common/stakewise"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/gorilla/mux"

	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/api/types"
)

// ===============
// === Factory ===
// ===============

type stakeWiseGetVaultsContextFactory struct {
	handler *StakeWiseHandler
}

func (f *stakeWiseGetVaultsContextFactory) Create(args url.Values) (*stakeWiseGetVaultsContext, error) {
	c := &stakeWiseGetVaultsContext{
		handler: f.handler,
	}
	inputErrs := []error{
		server.GetStringFromVars("deployment", args, &c.deployment),
	}
	return c, errors.Join(inputErrs...)
}

func (f *stakeWiseGetVaultsContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterQuerylessGet[*stakeWiseGetVaultsContext, api.NodeSetStakeWise_GetVaultsData](
		router, "get-vaults", f, f.handler.logger.Logger, f.handler.serviceProvider,
	)
}

// ===============
// === Context ===
// ===============
type stakeWiseGetVaultsContext struct {
	handler *StakeWiseHandler

	deployment string
}

func (c *stakeWiseGetVaultsContext) PrepareData(data *api.NodeSetStakeWise_GetVaultsData, opts *bind.TransactOpts) (types.ResponseStatus, error) {
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

	// Get the vaults
	ns := sp.GetNodeSetServiceManager()
	response, err := ns.StakeWise_GetVaults(ctx, c.deployment)
	if err != nil {
		if errors.Is(err, stakewise.ErrInvalidPermissions) {
			data.InvalidPermissions = true
			return types.ResponseStatus_Success, nil
		}
		return types.ResponseStatus_Error, err
	}

	data.Vaults = response
	return types.ResponseStatus_Success, nil
}
