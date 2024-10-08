package nodeset

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/nodeset-org/hyperdrive-daemon/common"
	ns_constellation "github.com/nodeset-org/hyperdrive-daemon/server/api/nodeset/constellation"
	ns_stakewise "github.com/nodeset-org/hyperdrive-daemon/server/api/nodeset/stakewise"
	"github.com/rocket-pool/node-manager-core/api/server"
	"github.com/rocket-pool/node-manager-core/log"
)

type NodeSetHandler struct {
	logger               *log.Logger
	ctx                  context.Context
	serviceProvider      common.IHyperdriveServiceProvider
	factories            []server.IContextFactory
	stakeWiseHandler     *ns_stakewise.StakeWiseHandler
	constellationHandler *ns_constellation.ConstellationHandler
}

func NewNodeSetHandler(logger *log.Logger, ctx context.Context, serviceProvider common.IHyperdriveServiceProvider) *NodeSetHandler {
	h := &NodeSetHandler{
		logger:          logger,
		ctx:             ctx,
		serviceProvider: serviceProvider,
	}
	h.factories = []server.IContextFactory{
		&nodeSetRegisterNodeContextFactory{h},
		&nodeSetGetRegistrationStatusContextFactory{h},
	}
	return h
}

func (h *NodeSetHandler) RegisterRoutes(router *mux.Router) {
	subrouter := router.PathPrefix("/nodeset").Subrouter()
	for _, factory := range h.factories {
		factory.RegisterRoute(subrouter)
	}

	// Register StakeWise routes
	h.stakeWiseHandler = ns_stakewise.NewStakeWiseHandler(h.logger, h.ctx, h.serviceProvider)
	h.stakeWiseHandler.RegisterRoutes(subrouter)

	// Register Constellation routes
	h.constellationHandler = ns_constellation.NewConstellationHandler(h.logger, h.ctx, h.serviceProvider)
	h.constellationHandler.RegisterRoutes(subrouter)
}
