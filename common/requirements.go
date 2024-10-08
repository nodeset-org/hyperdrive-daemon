package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	"github.com/rocket-pool/node-manager-core/eth"
	"github.com/rocket-pool/node-manager-core/log"
	"github.com/rocket-pool/node-manager-core/node/services"
	"github.com/rocket-pool/node-manager-core/utils"
)

// Settings
const (
	// Log keys
	PrimarySyncProgressKey  string = "primarySyncProgress"
	FallbackSyncProgressKey string = "fallbackSyncProgress"
	SyncProgressKey         string = "syncProgress"
	PrimaryErrorKey         string = "primaryError"
	FallbackErrorKey        string = "fallbackError"

	ethClientStatusRefreshInterval   time.Duration = 60 * time.Second
	ethClientSyncPollInterval        time.Duration = 5 * time.Second
	beaconClientSyncPollInterval     time.Duration = 5 * time.Second
	walletReadyCheckInterval         time.Duration = 15 * time.Second
	nodeSetRegistrationCheckInterval time.Duration = 15 * time.Second
)

var (
	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrNodeAddressNotSet error = errors.New("The node currently does not have an address set. Please run 'hyperdrive wallet restore-address' or 'hyperdrive wallet masquerade' and try again.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrNeedPassword error = errors.New("The node has a node wallet on disk but does not have the password for it loaded. Please run `hyperdrive wallet set-password` to load it.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrCantLoadWallet error = errors.New("The node has a node wallet and a password on disk but there was an error loading it - perhaps the password is incorrect? Please check the daemon logs for more information.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrNoKeystore error = errors.New("The node currently does not have a node wallet keystore. Please run 'hyperdrive wallet init' or 'hyperdrive wallet recover' and try again.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrNoAddress error = errors.New("The node currently does not have an address set. Please run 'hyperdrive wallet restore-address' or 'hyperdrive wallet masquerade' and try again.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrAddressMismatch error = errors.New("The node's wallet keystore does not match the node address. This node is currently in read-only mode.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrExecutionClientNotSynced error = errors.New("The Execution client is currently syncing. Please try again later.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrBeaconNodeNotSynced error = errors.New("The Beacon node is currently syncing. Please try again later.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrNotRegisteredWithNodeSet error = errors.New("The node is not registered with the Node Set. Please run 'hyperdrive nodeset register-node' and try again.")

	//lint:ignore ST1005 These are printed to the user and need to be in proper grammatical format
	ErrWalletNotReady error = errors.New("The node does not have a wallet ready yet. Please run 'hyperdrive wallet status' to learn more first.")
)

// ====================
// === Requirements ===
// ====================

func (sp *serviceProvider) RequireNodeAddress() error {
	status, err := sp.GetWallet().GetStatus()
	if err != nil {
		return err
	}
	if !status.Address.HasAddress {
		return ErrNodeAddressNotSet
	}
	return nil
}

func (sp *serviceProvider) RequireWalletReady() error {
	status, err := sp.GetWallet().GetStatus()
	if err != nil {
		return err
	}
	if !status.Wallet.IsLoaded {
		if status.Wallet.IsOnDisk {
			if !status.Password.IsPasswordSaved {
				return ErrNeedPassword
			}
			return ErrCantLoadWallet
		}
		return ErrNoKeystore
	}
	if !status.Address.HasAddress {
		return ErrNoAddress
	}
	if status.Wallet.WalletAddress != status.Address.NodeAddress {
		return ErrAddressMismatch
	}
	return nil
}

func (sp *serviceProvider) RequireEthClientSynced(ctx context.Context) error {
	synced, _, err := sp.checkExecutionClientStatus(ctx)
	if err != nil {
		return err
	}
	if synced {
		return nil
	}
	return ErrExecutionClientNotSynced
}

func (sp *serviceProvider) RequireBeaconClientSynced(ctx context.Context) error {
	synced, err := sp.checkBeaconClientStatus(ctx)
	if err != nil {
		return err
	}
	if synced {
		return nil
	}
	return ErrBeaconNodeNotSynced
}

func (sp *serviceProvider) RequireRegisteredWithNodeSet(ctx context.Context) error {
	status, err := sp.ns.GetRegistrationStatus(ctx)
	if err != nil {
		return err
	}
	switch status {
	case api.NodeSetRegistrationStatus_Registered:
		return nil
	case api.NodeSetRegistrationStatus_Unregistered:
		return ErrNotRegisteredWithNodeSet
	case api.NodeSetRegistrationStatus_NoWallet:
		return ErrWalletNotReady
	}
	return fmt.Errorf("unknown registration status [%v]", status)
}

// Wait for the Executon client to sync; timeout of 0 indicates no timeout
func (sp *serviceProvider) WaitEthClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitEthClientSynced(ctx, verbose)
	return err
}

// Wait for the Beacon client to sync; timeout of 0 indicates no timeout
func (sp *serviceProvider) WaitBeaconClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitBeaconClientSynced(ctx, verbose)
	return err
}

// Wait for the wallet to be ready
func (sp *serviceProvider) WaitForWallet(ctx context.Context) error {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	for {
		if sp.RequireWalletReady() == nil {
			return nil
		}

		logger.Info("Hyperdrive wallet not ready yet",
			slog.Duration("retry", walletReadyCheckInterval),
		)
		if utils.SleepWithCancel(ctx, walletReadyCheckInterval) {
			return nil
		}
	}
}

// Wait until the node has been registered with NodeSet.
// Returns true if the context was cancelled and the caller should exit.
func (sp *serviceProvider) WaitForNodeSetRegistration(ctx context.Context) bool {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Wait for NodeSet registration
	ns := sp.GetNodeSetServiceManager()
	for {
		status, err := ns.GetRegistrationStatus(ctx)
		if status == api.NodeSetRegistrationStatus_Registered {
			return false
		}

		var msg string
		switch status {
		case api.NodeSetRegistrationStatus_NoWallet:
			msg = "Can't check NodeSet registration status until node has a wallet"
		case api.NodeSetRegistrationStatus_Unregistered:
			msg = "Not registered with NodeSet yet"
		case api.NodeSetRegistrationStatus_Unknown:
			msg = fmt.Sprintf("Can't check NodeSet registration status (%s)", err.Error())
		}
		logger.Info(msg,
			slog.Duration("retry", nodeSetRegistrationCheckInterval),
		)
		if utils.SleepWithCancel(ctx, nodeSetRegistrationCheckInterval) {
			return true
		}
	}
}

// ===============
// === Helpers ===
// ===============

// Check if the primary and fallback Execution clients are synced
// TODO: Move this into ec-manager and stop exposing the primary and fallback directly...
func (sp *serviceProvider) checkExecutionClientStatus(ctx context.Context) (bool, eth.IExecutionClient, error) {
	// Check the EC status
	ecMgr := sp.GetEthClient()
	mgrStatus := ecMgr.CheckStatus(ctx, true) // Always check the chain ID for now
	if ecMgr.IsPrimaryReady() {
		return true, nil, nil
	}

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// If the primary isn't synced but there's a fallback and it is, return true
	if ecMgr.IsFallbackReady() {
		if mgrStatus.PrimaryClientStatus.Error != "" {
			logger.Warn("Primary execution client is unavailable using fallback execution client...", slog.String(log.ErrorKey, mgrStatus.PrimaryClientStatus.Error))
		} else {
			logger.Warn("Primary execution client is still syncing, using fallback execution client...", slog.Float64(PrimarySyncProgressKey, mgrStatus.PrimaryClientStatus.SyncProgress*100))
		}
		return true, nil, nil
	}

	// If neither is synced, go through the status to figure out what to do

	// Is the primary working and syncing? If so, wait for it
	if mgrStatus.PrimaryClientStatus.IsWorking && mgrStatus.PrimaryClientStatus.Error == "" {
		logger.Error("Fallback execution client is not configured or unavailable, waiting for primary execution client to finish syncing", slog.Float64(PrimarySyncProgressKey, mgrStatus.PrimaryClientStatus.SyncProgress*100))
		return false, ecMgr.GetPrimaryClient(), nil
	}

	// Is the fallback working and syncing? If so, wait for it
	if mgrStatus.FallbackEnabled && mgrStatus.FallbackClientStatus.IsWorking && mgrStatus.FallbackClientStatus.Error == "" {
		logger.Error("Primary execution client is unavailable, waiting for the fallback execution client to finish syncing", slog.String(PrimaryErrorKey, mgrStatus.PrimaryClientStatus.Error), slog.Float64(FallbackSyncProgressKey, mgrStatus.FallbackClientStatus.SyncProgress*100))
		return false, ecMgr.GetFallbackClient(), nil
	}

	// If neither client is working, report the errors
	if mgrStatus.FallbackEnabled {
		return false, nil, fmt.Errorf("Primary execution client is unavailable (%s) and fallback execution client is unavailable (%s), no execution clients are ready.", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.Error)
	}

	return false, nil, fmt.Errorf("Primary execution client is unavailable (%s) and no fallback execution client is configured.", mgrStatus.PrimaryClientStatus.Error)
}

// Check if the primary and fallback Beacon clients are synced
func (sp *serviceProvider) checkBeaconClientStatus(ctx context.Context) (bool, error) {
	// Check the BC status
	bcMgr := sp.GetBeaconClient()
	mgrStatus := bcMgr.CheckStatus(ctx, true) // Always check the chain ID for now
	if bcMgr.IsPrimaryReady() {
		return true, nil
	}

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// If the primary isn't synced but there's a fallback and it is, return true
	if bcMgr.IsFallbackReady() {
		if mgrStatus.PrimaryClientStatus.Error != "" {
			logger.Warn("Primary Beacon Node is unavailable, using fallback Beacon Node...", slog.String(PrimaryErrorKey, mgrStatus.PrimaryClientStatus.Error))
		} else {
			logger.Warn("Primary Beacon Node is still syncing, using fallback Beacon Node...", slog.Float64(PrimarySyncProgressKey, mgrStatus.PrimaryClientStatus.SyncProgress*100))
		}
		return true, nil
	}

	// If neither is synced, go through the status to figure out what to do

	// Is the primary working and syncing? If so, wait for it
	if mgrStatus.PrimaryClientStatus.IsWorking && mgrStatus.PrimaryClientStatus.Error == "" {
		logger.Error("Fallback Beacon Node is not configured or unavailable, waiting for primary Beacon Node to finish syncing...", slog.Float64(PrimarySyncProgressKey, mgrStatus.PrimaryClientStatus.SyncProgress*100))
		return false, nil
	}

	// Is the fallback working and syncing? If so, wait for it
	if mgrStatus.FallbackEnabled && mgrStatus.FallbackClientStatus.IsWorking && mgrStatus.FallbackClientStatus.Error == "" {
		logger.Error("Primary Beacon Node is unavailable, waiting for the fallback Beacon Node to finish syncing...", slog.String(PrimaryErrorKey, mgrStatus.PrimaryClientStatus.Error), slog.Float64(FallbackSyncProgressKey, mgrStatus.FallbackClientStatus.SyncProgress*100))
		return false, nil
	}

	// If neither client is working, report the errors
	if mgrStatus.FallbackEnabled {
		return false, fmt.Errorf("Primary Beacon Node is unavailable (%s) and fallback Beacon Node is unavailable (%s), no Beacon Nodes are ready.", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.Error)
	}

	return false, fmt.Errorf("Primary Beacon Node is unavailable (%s) and no fallback Beacon Node is configured.", mgrStatus.PrimaryClientStatus.Error)
}

// Wait for the primary or fallback Execution client to be synced
func (sp *serviceProvider) waitEthClientSynced(ctx context.Context, verbose bool) (bool, error) {
	synced, clientToCheck, err := sp.checkExecutionClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get wait start time
	startTime := time.Now()

	// Get EC status refresh time
	ecRefreshTime := startTime

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Wait for sync
	for {
		// Check if the EC status needs to be refreshed
		if time.Since(ecRefreshTime) > ethClientStatusRefreshInterval {
			logger.Info("Refreshing primary / fallback execution client status...")
			ecRefreshTime = time.Now()
			synced, clientToCheck, err = sp.checkExecutionClientStatus(ctx)
			if err != nil {
				return false, err
			}
			if synced {
				return true, nil
			}
		}

		// Get sync progress
		progress, err := clientToCheck.SyncProgress(ctx)
		if err != nil {
			return false, err
		}

		// Check sync progress
		if progress != nil {
			if verbose {
				p := float64(progress.CurrentBlock-progress.StartingBlock) / float64(progress.HighestBlock-progress.StartingBlock)
				if p > 1 {
					logger.Info("Execution client syncing...")
				} else {
					logger.Info("Execution client syncing...", slog.Float64(SyncProgressKey, p*100))
				}
			}
		} else {
			// Eth 1 client is not in "syncing" state but may be behind head
			// Get the latest block it knows about and make sure it's recent compared to system clock time
			isUpToDate, _, err := services.IsSyncWithinThreshold(clientToCheck)
			if err != nil {
				return false, err
			}
			// Only return true if the last reportedly known block is within our defined threshold
			if isUpToDate {
				return true, nil
			}
		}

		// Pause before next poll
		if utils.SleepWithCancel(ctx, ethClientSyncPollInterval) {
			return false, nil
		}
	}
}

// Wait for the primary or fallback Beacon client to be synced
func (sp *serviceProvider) waitBeaconClientSynced(ctx context.Context, verbose bool) (bool, error) {
	synced, err := sp.checkBeaconClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get wait start time
	startTime := time.Now()

	// Get BC status refresh time
	bcRefreshTime := startTime

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Wait for sync
	for {
		// Check if the BC status needs to be refreshed
		if time.Since(bcRefreshTime) > ethClientStatusRefreshInterval {
			logger.Info("Refreshing primary / fallback Beacon Node status...")
			bcRefreshTime = time.Now()
			synced, err = sp.checkBeaconClientStatus(ctx)
			if err != nil {
				return false, err
			}
			if synced {
				return true, nil
			}
		}

		// Get sync status
		syncStatus, err := sp.GetBeaconClient().GetSyncStatus(ctx)
		if err != nil {
			return false, err
		}

		// Check sync status
		if syncStatus.Syncing {
			if verbose {
				logger.Info("Beacon Node syncing...", slog.Float64(SyncProgressKey, syncStatus.Progress*100))
			}
		} else {
			return true, nil
		}

		// Pause before next poll
		if utils.SleepWithCancel(ctx, beaconClientSyncPollInterval) {
			return false, nil
		}
	}
}
