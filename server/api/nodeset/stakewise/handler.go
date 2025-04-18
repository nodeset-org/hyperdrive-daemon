package ns_stakewise

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/log"
)

type StakeWiseHandler struct {
	logger          *log.Logger
	ctx             context.Context
	serviceProvider common.IHyperdriveServiceProvider
	factories       []server.IContextFactory
}

func NewStakeWiseHandler(logger *log.Logger, ctx context.Context, serviceProvider common.IHyperdriveServiceProvider) *StakeWiseHandler {
	h := &StakeWiseHandler{
		logger:          logger,
		ctx:             ctx,
		serviceProvider: serviceProvider,
	}
	h.factories = []server.IContextFactory{
		&stakeWiseGetValidatorsInfoContextFactory{h},
		&stakeWiseGetRegisteredValidatorsContextFactory{h},
		&stakeWiseGetValidatorManagerSignatureContextFactory{h},
		&stakeWiseGetVaultsContextFactory{h},
	}
	return h
}

func (h *StakeWiseHandler) RegisterRoutes(router *mux.Router) {
	subrouter := router.PathPrefix("/stakewise").Subrouter()
	for _, factory := range h.factories {
		factory.RegisterRoute(subrouter)
	}
}
