package wallet

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	common "github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	hdcommon "github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/api/types"
	"github.com/rocket-pool/node-manager-core/utils/input"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
)

// ===============
// === Factory ===
// ===============

type walletGenerateDepositDataContextFactory struct {
	handler *WalletHandler
}

func (f *walletGenerateDepositDataContextFactory) Create(args url.Values) (*walletGenerateDepositDataContext, error) {
	c := &walletGenerateDepositDataContext{
		handler: f.handler,
	}
	inputErrs := []error{
		server.ValidateArg("address", args, input.ValidateAddress, &c.minipoolAddress),
	}
	return c, errors.Join(inputErrs...)
}

func (f *walletGenerateDepositDataContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterQuerylessGet[*walletGenerateDepositDataContext, api.WalletGenerateDepositData](
		router, "generate-deposit-data", f, f.handler.logger.Logger, f.handler.serviceProvider.ServiceProvider,
	)
}

// ===============
// === Context ===
// ===============

type walletGenerateDepositDataContext struct {
	handler         *WalletHandler
	minipoolAddress common.Address
}

func (c *walletGenerateDepositDataContext) PrepareData(data *api.WalletGenerateDepositData, opts *bind.TransactOpts) (types.ResponseStatus, error) {
	sp := c.handler.serviceProvider
	w := sp.GetWallet()
	ddm, err := hdcommon.NewDepositDataManager(sp)
	if err != nil {
		return types.ResponseStatus_Error, fmt.Errorf("error instantiating new deposit data manager: %w", err)
	}
	privateKeyBytes, err := w.GetNodePrivateKeyBytes()
	if err != nil {
		return types.ResponseStatus_Error, fmt.Errorf("error getting node private key bytes: %w", err)
	}
	blsPrivateKey, err := eth2types.BLSPrivateKeyFromBytes(privateKeyBytes)
	if err != nil {
		return types.ResponseStatus_Error, fmt.Errorf("error getting BLS private key from bytes: %w", err)
	}
	blsPrivateKeys := []*eth2types.BLSPrivateKey{blsPrivateKey}

	depositData, err := ddm.GenerateDepositData(blsPrivateKeys, c.minipoolAddress)
	if err != nil {
		return types.ResponseStatus_Error, fmt.Errorf("error generating deposit data: %w", err)
	}
	data.PublicKey = depositData[0].PublicKey
	data.Signature = depositData[0].Signature
	data.DepositDataRoot = depositData[0].DepositDataRoot
	return types.ResponseStatus_Success, nil
}
