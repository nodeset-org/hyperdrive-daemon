package testing

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nodeset-org/hyperdrive-daemon/client"
	"github.com/nodeset-org/hyperdrive-daemon/common"
	"github.com/nodeset-org/hyperdrive-daemon/server"
	hdconfig "github.com/nodeset-org/hyperdrive-daemon/shared/config"
	nsserver "github.com/nodeset-org/nodeset-client-go/server-mock/server"
	"github.com/nodeset-org/osha"
	"github.com/rocket-pool/node-manager-core/log"
	"github.com/rocket-pool/node-manager-core/node/services"
)

// HyperdriveTestManager provides bootstrapping and a test service provider, useful for testing
type HyperdriveTestManager struct {
	*osha.TestManager

	// The service provider for the test environment
	serviceProvider *common.ServiceProvider

	// The mock for the nodeset.io service
	nodesetMock *nsserver.NodeSetMockServer

	// The Hyperdrive Daemon server
	serverMgr *server.ServerManager

	// The Hyperdrive Daemon client
	apiClient *client.ApiClient

	// Wait groups for graceful shutdown
	hdWg *sync.WaitGroup
	nsWg *sync.WaitGroup
}

// Creates a new HyperdriveTestManager instance. Requires management of your own nodeset.io server mock.
// `address` is the address to bind the Hyperdrive daemon to.
func NewHyperdriveTestManager(address string, cfg *hdconfig.HyperdriveConfig, resources *hdconfig.HyperdriveResources, nsServer *nsserver.NodeSetMockServer, nsWaitGroup *sync.WaitGroup) (*HyperdriveTestManager, error) {
	tm, err := osha.NewTestManager()
	if err != nil {
		return nil, fmt.Errorf("error creating test manager: %w", err)
	}
	return newHyperdriveTestManagerImpl(address, tm, cfg, resources, nsServer, nsWaitGroup)
}

// Creates a new HyperdriveTestManager instance with default test artifacts.
// `hyperdriveAddress` is the address to bind the Hyperdrive daemon to.
// `nodesetAddress` is the address to bind the nodeset.io server to.
func NewHyperdriveTestManagerWithDefaults(hyperdriveAddress string, nodesetAddress string) (*HyperdriveTestManager, error) {
	tm, err := osha.NewTestManager()
	if err != nil {
		return nil, fmt.Errorf("error creating test manager: %w", err)
	}

	// Make the nodeset.io mock server
	nsWg := &sync.WaitGroup{}
	nodesetMock, err := nsserver.NewNodeSetMockServer(tm.GetLogger(), nodesetAddress, 0)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating nodeset mock server: %v", err)
	}
	err = nodesetMock.Start(nsWg)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error starting nodeset mock server: %v", err)
	}

	// Make a new Hyperdrive config
	testDir := tm.GetTestDir()
	beaconCfg := tm.GetBeaconMockManager().GetConfig()
	resources := GetTestResources(beaconCfg, fmt.Sprintf("http://%s:%d/api/", nodesetAddress, nodesetMock.GetPort()))
	cfg := hdconfig.NewHyperdriveConfigForNetwork(testDir, hdconfig.Network_LocalTest, resources)
	cfg.Network.Value = hdconfig.Network_LocalTest

	// Make the test manager
	return newHyperdriveTestManagerImpl(hyperdriveAddress, tm, cfg, resources, nodesetMock, nsWg)
}

// Implementation for creating a new HyperdriveTestManager
func newHyperdriveTestManagerImpl(address string, tm *osha.TestManager, cfg *hdconfig.HyperdriveConfig, resources *hdconfig.HyperdriveResources, nsServer *nsserver.NodeSetMockServer, nsWaitGroup *sync.WaitGroup) (*HyperdriveTestManager, error) {
	// Make managers
	beaconCfg := tm.GetBeaconMockManager().GetConfig()
	ecManager := services.NewExecutionClientManager(tm.GetExecutionClient(), uint(beaconCfg.ChainID), time.Minute)
	bnManager := services.NewBeaconClientManager(tm.GetBeaconClient(), uint(beaconCfg.ChainID), time.Minute)

	// Make a new service provider
	serviceProvider, err := common.NewServiceProviderFromCustomServices(
		cfg,
		resources,
		ecManager,
		bnManager,
		tm.GetDockerMockManager(),
	)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating service provider: %v", err)
	}

	// Make sure the data and modules directories exist
	dataDir := cfg.UserDataPath.Value
	moduleDir := filepath.Join(dataDir, hdconfig.ModulesName)
	err = os.MkdirAll(moduleDir, 0755)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating data and modules directories [%s]: %v", moduleDir, err)
	}

	// Create the server
	hdWg := &sync.WaitGroup{}
	serverMgr, err := server.NewServerManager(serviceProvider, address, 0, hdWg)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating hyperdrive server: %v", err)
	}

	// Create the client
	urlString := fmt.Sprintf("http://%s:%d/%s", address, serverMgr.GetPort(), hdconfig.HyperdriveApiClientRoute)
	url, err := url.Parse(urlString)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error parsing client URL [%s]: %v", urlString, err)
	}
	apiClient := client.NewApiClient(url, tm.GetLogger(), nil)

	// Return
	m := &HyperdriveTestManager{
		TestManager:     tm,
		serviceProvider: serviceProvider,
		nodesetMock:     nsServer,
		serverMgr:       serverMgr,
		apiClient:       apiClient,
		hdWg:            hdWg,
		nsWg:            nsWaitGroup,
	}
	return m, nil
}

// Returns the service provider for the test environment
func (m *HyperdriveTestManager) GetServiceProvider() *common.ServiceProvider {
	return m.serviceProvider
}

// Get the nodeset.io mock server
func (m *HyperdriveTestManager) GetNodeSetMockServer() *nsserver.NodeSetMockServer {
	return m.nodesetMock
}

// Returns the Hyperdrive Daemon server
func (m *HyperdriveTestManager) GetServerManager() *server.ServerManager {
	return m.serverMgr
}

// Returns the Hyperdrive Daemon client
func (m *HyperdriveTestManager) GetApiClient() *client.ApiClient {
	return m.apiClient
}

// Closes the Hyperdrive test manager, shutting down the daemon
func (m *HyperdriveTestManager) Close() error {
	if m.nodesetMock != nil {
		err := m.nodesetMock.Stop()
		if err != nil {
			m.GetLogger().Warn("WARNING: nodeset server mock didn't shutdown cleanly", log.Err(err))
		}
		m.nsWg.Wait()
		m.TestManager.GetLogger().Info("Stopped nodeset.io mock server")
		m.nodesetMock = nil
	}
	if m.serverMgr != nil {
		m.serverMgr.Stop()
		m.hdWg.Wait()
		m.TestManager.GetLogger().Info("Stopped daemon API server")
		m.serverMgr = nil
	}
	if m.TestManager != nil {
		err := m.TestManager.Close()
		m.TestManager = nil
		return err
	}
	return nil
}

// ==========================
// === Internal Functions ===
// ==========================

// Closes the OSHA test manager, logging any errors
func closeTestManager(tm *osha.TestManager) {
	err := tm.Close()
	if err != nil {
		tm.GetLogger().Error("Error closing test manager", log.Err(err))
	}
}
