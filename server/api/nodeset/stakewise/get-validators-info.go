package ns_stakewise

import (
	"errors"
	"net/url"

	hdcommon "github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	"github.com/rocket-pool/node-manager-core/utils/input"

	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/api/types"
)

// ===============
// === Factory ===
// ===============

type stakeWiseGetValidatorsInfoContextFactory struct {
	handler *StakeWiseHandler
}

func (f *stakeWiseGetValidatorsInfoContextFactory) Create(args url.Values) (*stakeWiseGetValidatorsInfoContext, error) {
	c := &stakeWiseGetValidatorsInfoContext{
		handler: f.handler,
	}
	inputErrs := []error{
		server.GetStringFromVars("deployment", args, &c.deployment),
		server.ValidateArg("vault", args, input.ValidateAddress, &c.vault),
	}
	return c, errors.Join(inputErrs...)
}

func (f *stakeWiseGetValidatorsInfoContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterQuerylessGet[*stakeWiseGetValidatorsInfoContext, api.NodeSetStakeWise_GetValidatorsInfoData](
		router, "get-validators-info", f, f.handler.logger.Logger, f.handler.serviceProvider,
	)
}

// ===============
// === Context ===
// ===============
type stakeWiseGetValidatorsInfoContext struct {
	handler *StakeWiseHandler

	deployment string
	vault      common.Address
}

func (c *stakeWiseGetValidatorsInfoContext) PrepareData(data *api.NodeSetStakeWise_GetValidatorsInfoData, opts *bind.TransactOpts) (types.ResponseStatus, error) {
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

	// Get the validators info for this node
	ns := sp.GetNodeSetServiceManager()
	response, err := ns.StakeWise_GetValidatorsInfoForNodeAccount(ctx, c.deployment, c.vault)
	if err != nil {
		return types.ResponseStatus_Error, err
	}

	data.Active = int(response.Active)
	data.Max = int(response.Max)
	data.Available = int(response.Available)
	return types.ResponseStatus_Success, nil
}
