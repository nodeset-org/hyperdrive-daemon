package services

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
	"github.com/rocket-pool/node-manager-core/wallet"
)

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
	ErrExecutionClientNotSynced error = errors.New("The Execution client is currently syncing. Please try again later.")
	ErrBeaconNodeNotSynced      error = errors.New("The Beacon node is currently syncing. Please try again later.")
	ErrNotRegisteredWithNodeSet error = errors.New("The node is not registered with the Node Set. Please run 'hyperdrive nodeset register-node' and try again.")
	ErrWalletNotReady           error = errors.New("The node does not have a wallet ready yet. Please run 'hyperdrive wallet status' to learn more first.")
)

func (sp *moduleServiceProvider) RequireNodeAddress(status wallet.WalletStatus) error {
	if !status.Address.HasAddress {
		return ErrNodeAddressNotSet
	}
	return nil
}

func (sp *moduleServiceProvider) RequireWalletReady(status wallet.WalletStatus) error {
	return CheckIfWalletReady(status)
}

func (sp *moduleServiceProvider) RequireEthClientSynced(ctx context.Context) error {
	synced, _, err := sp.checkExecutionClientStatus(ctx)
	if err != nil {
		return err
	}
	if synced {
		return nil
	}
	return ErrExecutionClientNotSynced
}

func (sp *moduleServiceProvider) RequireBeaconClientSynced(ctx context.Context) error {
	synced, err := sp.checkBeaconClientStatus(ctx)
	if err != nil {
		return err
	}
	if synced {
		return nil
	}
	return ErrBeaconNodeNotSynced
}

func (sp *moduleServiceProvider) RequireRegisteredWithNodeSet(ctx context.Context) error {
	response, err := sp.hdClient.NodeSet.GetRegistrationStatus()
	if err != nil {
		return err
	}
	switch response.Data.Status {
	case api.NodeSetRegistrationStatus_Registered:
		return nil
	case api.NodeSetRegistrationStatus_Unregistered:
		return ErrNotRegisteredWithNodeSet
	case api.NodeSetRegistrationStatus_NoWallet:
		return ErrWalletNotReady
	}
	return fmt.Errorf("unknown registration status [%v]", response.Data.Status)
}

// Wait for the Executon client to sync; timeout of 0 indicates no timeout
func (sp *moduleServiceProvider) WaitEthClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitEthClientSynced(ctx, verbose)
	return err
}

// Wait for the Beacon client to sync; timeout of 0 indicates no timeout
func (sp *moduleServiceProvider) WaitBeaconClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitBeaconClientSynced(ctx, verbose)
	return err
}

// Wait for Hyperdrive to have a node address assigned
func (sp *moduleServiceProvider) WaitForNodeAddress(ctx context.Context) (*wallet.WalletStatus, error) {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	for {
		hdWalletStatus, err := sp.GetHyperdriveClient().Wallet.Status()
		if err != nil {
			return nil, fmt.Errorf("error getting Hyperdrive wallet status: %w", err)
		}

		status := hdWalletStatus.Data.WalletStatus
		if status.Address.HasAddress {
			return &status, nil
		}

		logger.Info("Node address not present yet",
			slog.Duration("retry", walletReadyCheckInterval),
		)
		if utils.SleepWithCancel(ctx, walletReadyCheckInterval) {
			return nil, nil
		}
	}
}

// Wait for the Hyperdrive wallet to be ready
func (sp *moduleServiceProvider) WaitForWallet(ctx context.Context) (*wallet.WalletStatus, error) {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	for {
		hdWalletStatus, err := sp.GetHyperdriveClient().Wallet.Status()
		if err != nil {
			return nil, fmt.Errorf("error getting Hyperdrive wallet status: %w", err)
		}

		if CheckIfWalletReady(hdWalletStatus.Data.WalletStatus) == nil {
			return &hdWalletStatus.Data.WalletStatus, nil
		}

		logger.Info("Hyperdrive wallet not ready yet",
			slog.Duration("retry", walletReadyCheckInterval),
		)
		if utils.SleepWithCancel(ctx, walletReadyCheckInterval) {
			return nil, nil
		}
	}
}

// Wait until the node has been registered with NodeSet.
// Returns true if the context was cancelled and the caller should exit.
func (sp *moduleServiceProvider) WaitForNodeSetRegistration(ctx context.Context) bool {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Wait for NodeSet registration
	hd := sp.GetHyperdriveClient()
	for {
		var msg string
		response, err := hd.NodeSet.GetRegistrationStatus()
		if err != nil {
			msg = fmt.Sprintf("Can't check NodeSet registration status (%s)", err.Error())
		} else {
			switch response.Data.Status {
			case api.NodeSetRegistrationStatus_NoWallet:
				msg = "Can't check NodeSet registration status until node has a wallet"
			case api.NodeSetRegistrationStatus_Registered:
				return false
			case api.NodeSetRegistrationStatus_Unregistered:
				msg = "Not registered with NodeSet yet"
			case api.NodeSetRegistrationStatus_Unknown:
				msg = fmt.Sprintf("Can't check NodeSet registration status (%s)", response.Data.ErrorMessage)
			}
		}

		logger.Info(msg,
			slog.Duration("retry", nodeSetRegistrationCheckInterval),
		)
		if utils.SleepWithCancel(ctx, nodeSetRegistrationCheckInterval) {
			return true
		}
	}
}

// Check if the primary and fallback Execution clients are synced
// TODO: Move this into ec-manager and stop exposing the primary and fallback directly...
func (sp *moduleServiceProvider) checkExecutionClientStatus(ctx context.Context) (bool, eth.IExecutionClient, error) {
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
func (sp *moduleServiceProvider) checkBeaconClientStatus(ctx context.Context) (bool, error) {
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
func (sp *moduleServiceProvider) waitEthClientSynced(ctx context.Context, verbose bool) (bool, error) {
	synced, clientToCheck, err := sp.checkExecutionClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get EC status refresh time
	ecRefreshTime := time.Now()

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
		time.Sleep(ethClientSyncPollInterval)
	}
}

// Wait for the primary or fallback Beacon client to be synced
func (sp *moduleServiceProvider) waitBeaconClientSynced(ctx context.Context, verbose bool) (bool, error) {
	synced, err := sp.checkBeaconClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get BC status refresh time
	bcRefreshTime := time.Now()

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
		time.Sleep(beaconClientSyncPollInterval)
	}
}
