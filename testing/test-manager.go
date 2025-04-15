package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nodeset-org/hyperdrive-daemon/common"
	hdconfig "github.com/nodeset-org/hyperdrive-daemon/shared/config"
	nsserver "github.com/nodeset-org/nodeset-client-go/server-mock/server"
	"github.com/nodeset-org/osha"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/log"
	"github.com/rocket-pool/node-manager-core/node/services"
)

// A custom provisioning function that can alter or update the network settings used by the test manager prior to starting the Hyperdrive daemon
type NetworkSettingsProvisioner func(*config.NetworkSettings) *config.NetworkSettings

// HyperdriveTestManager provides bootstrapping and a test service provider, useful for testing
type HyperdriveTestManager struct {
	*osha.TestManager

	// The Hyperdrive node owned by this test manager
	node *HyperdriveNode

	// The mock for the nodeset.io service
	nodesetMock *nsserver.NodeSetMockServer

	// Snapshot ID from the baseline - the initial state of the nodeset.io service prior to running any tests
	baselineSnapshotID string

	// Map of which services were captured during a snapshot
	snapshotServiceMap map[string]Service

	// Wait groups for graceful shutdown
	nsWg *sync.WaitGroup
}

// Creates a new HyperdriveTestManager instance. Requires management of your own nodeset.io server mock.
// `address` is the address to bind the Hyperdrive daemon to.
func NewHyperdriveTestManager(address string, port uint, cfg *hdconfig.HyperdriveConfig, resources *hdconfig.MergedResources, nsServer *nsserver.NodeSetMockServer) (*HyperdriveTestManager, error) {
	tm, err := osha.NewTestManager()
	if err != nil {
		return nil, fmt.Errorf("error creating test manager: %w", err)
	}
	module, err := newHyperdriveTestManagerImpl(address, tm, cfg, resources, nsServer, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Hyperdrive test manager: %w", err)
	}
	// Register the module
	tm.RegisterModule(module)

	// Take a baseline snapshot
	baselineSnapshot, err := tm.CreateSnapshot()
	if err != nil {
		return nil, fmt.Errorf("error creating baseline snapshot: %w", err)
	}
	module.baselineSnapshotID = baselineSnapshot
	return module, nil
}

// Creates a new HyperdriveTestManager instance with default test artifacts.
func NewHyperdriveTestManagerWithDefaults(netSettingsProvisioner NetworkSettingsProvisioner) (*HyperdriveTestManager, error) {
	tm, err := osha.NewTestManager()
	if err != nil {
		return nil, fmt.Errorf("error creating test manager: %w", err)
	}

	// Make the nodeset.io mock server
	nsWg := &sync.WaitGroup{}
	nodesetMock, err := nsserver.NewNodeSetMockServer(tm.GetLogger(), "localhost", 0)
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
	networkSettings := GetDefaultTestNetworkSettings(beaconCfg)
	networkSettings = netSettingsProvisioner(networkSettings)
	resources := getTestResources(networkSettings.NetworkResources, fmt.Sprintf("http://%s:%d/api/", "localhost", nodesetMock.GetPort()))
	hdNetSettings := &hdconfig.HyperdriveSettings{
		NetworkSettings:     networkSettings,
		HyperdriveResources: resources.HyperdriveResources,
	}
	cfg, err := hdconfig.NewHyperdriveConfigForNetwork(testDir, []*hdconfig.HyperdriveSettings{hdNetSettings}, hdconfig.Network_LocalTest)
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating Hyperdrive config: %v", err)
	}
	cfg.Network.Value = hdconfig.Network_LocalTest
	cfg.ApiPort.Value = 0

	// Make the test manager
	module, err := newHyperdriveTestManagerImpl("localhost", tm, cfg, resources, nodesetMock, nsWg)
	if err != nil {
		return nil, fmt.Errorf("error creating Hyperdrive test manager: %w", err)
	}
	tm.RegisterModule(module)
	baselineSnapshot, err := tm.CreateSnapshot()
	if err != nil {
		return nil, fmt.Errorf("error creating baseline snapshot: %w", err)
	}
	module.baselineSnapshotID = baselineSnapshot

	if err != nil {
		return nil, fmt.Errorf("error registering module: %w", err)
	}
	return module, nil
}

// Implementation for creating a new HyperdriveTestManager
func newHyperdriveTestManagerImpl(address string, tm *osha.TestManager, cfg *hdconfig.HyperdriveConfig, resources *hdconfig.MergedResources, nsServer *nsserver.NodeSetMockServer, nsWaitGroup *sync.WaitGroup) (*HyperdriveTestManager, error) {
	// Make managers
	beaconCfg := tm.GetBeaconMockManager().GetConfig()
	ecManager := services.NewExecutionClientManager(tm.GetExecutionClient(), uint(beaconCfg.ChainID), time.Minute)
	bnManager := services.NewBeaconClientManager(tm.GetBeaconClient(), uint(beaconCfg.ChainID), time.Minute)

	// Make a new service provider
	serviceProvider, err := common.NewHyperdriveServiceProviderFromCustomServices(
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

	// Make the Hyperdrive node
	node, err := newHyperdriveNode(serviceProvider, address, tm.GetLogger())
	if err != nil {
		closeTestManager(tm)
		return nil, fmt.Errorf("error creating Hyperdrive node: %v", err)
	}

	// Return
	m := &HyperdriveTestManager{
		TestManager:        tm,
		node:               node,
		nodesetMock:        nsServer,
		nsWg:               nsWaitGroup,
		snapshotServiceMap: map[string]Service{},
	}

	return m, nil
}

// Closes the Hyperdrive test manager, shutting down the daemon
func (m *HyperdriveTestManager) CloseModule() error {
	if m.nodesetMock != nil {
		// Check if we're managing the service - if so just stop it
		if m.nsWg != nil {
			err := m.nodesetMock.Stop()
			if err != nil {
				m.GetLogger().Warn("WARNING: nodeset server mock didn't shutdown cleanly", log.Err(err))
			}
			m.nsWg.Wait()
			m.TestManager.GetLogger().Info("Stopped nodeset.io mock server")
		} else {
			err := m.nodesetMock.GetManager().RevertToSnapshot(m.baselineSnapshotID)
			if err != nil {
				m.GetLogger().Warn("WARNING: error reverting nodeset server mock to baseline", log.Err(err))
			} else {
				m.TestManager.GetLogger().Info("Reverted nodeset.io mock server to baseline snapshot")
			}
		}
		m.nodesetMock = nil
	}
	err := m.node.Close()
	if err != nil {
		return fmt.Errorf("error closing Hyperdrive node: %w", err)
	}
	if m.TestManager != nil {
		err := m.TestManager.Close()
		m.TestManager = nil
		return err
	}
	return nil
}

// ===============
// === Getters ===
// ===============

// Get the Hyperdrive node
func (m *HyperdriveTestManager) GetNode() *HyperdriveNode {
	return m.node
}

// Get the nodeset.io mock server
func (m *HyperdriveTestManager) GetNodeSetMockServer() *nsserver.NodeSetMockServer {
	return m.nodesetMock
}

func (m *HyperdriveTestManager) GetModuleName() string {
	return "hyperdrive-daemon"
}

// ====================
// === Snapshotting ===
// ====================

// Takes a snapshot of the service states
func (m *HyperdriveTestManager) DependsOnBaseline() error {
	err := m.RevertSnapshot(m.baselineSnapshotID)
	if err != nil {
		return fmt.Errorf("error reverting to baseline snapshot %s: %w", m.baselineSnapshotID, err)
	}
	return nil
}

// Takes a snapshot of the service states
func (m *HyperdriveTestManager) TakeModuleSnapshot() (any, error) {
	snapshotName := uuid.New().String()
	m.nodesetMock.GetManager().TakeSnapshot(snapshotName)
	return snapshotName, nil
}

// Revert the services to a snapshot state
func (m *HyperdriveTestManager) RevertModuleToSnapshot(moduleState any) error {
	err := m.nodesetMock.GetManager().RevertToSnapshot(moduleState.(string))
	if err != nil {
		return fmt.Errorf("error reverting the nodeset.io mock to snapshot %s: %w", moduleState, err)
	}

	wallet := m.node.sp.GetWallet()
	err = wallet.Reload(m.GetLogger())
	if err != nil {
		return fmt.Errorf("error reloading wallet: %w", err)
	}
	return nil
}

// Closes the OSHA test manager, logging any errors
func closeTestManager(tm *osha.TestManager) {
	err := tm.Close()
	if err != nil {
		tm.GetLogger().Error("Error closing test manager", log.Err(err))
	}
}
