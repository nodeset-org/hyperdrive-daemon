package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hdconfig "github.com/nodeset-org/hyperdrive-daemon/shared/config"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	apiv3 "github.com/nodeset-org/nodeset-client-go/api-v3"
	v3constellation "github.com/nodeset-org/nodeset-client-go/api-v3/constellation"
	v3stakewise "github.com/nodeset-org/nodeset-client-go/api-v3/stakewise"
	nscommon "github.com/nodeset-org/nodeset-client-go/common"
	"github.com/nodeset-org/nodeset-client-go/common/core"
	"github.com/nodeset-org/nodeset-client-go/common/stakewise"
	"github.com/rocket-pool/node-manager-core/beacon"
	"github.com/rocket-pool/node-manager-core/log"
	"github.com/rocket-pool/node-manager-core/node/wallet"
	"github.com/rocket-pool/node-manager-core/utils"
)

// NodeSetServiceManager is a manager for interactions with the NodeSet service
type NodeSetServiceManager struct {
	// The node wallet
	wallet *wallet.Wallet

	// Resources for the current network
	resources *hdconfig.MergedResources

	// Client for the v3 API
	v3Client *apiv3.NodeSetClient

	// The current session token
	sessionToken string

	// The node wallet's registration status
	nodeRegistrationStatus api.NodeSetRegistrationStatus

	// Mutex for the registration status
	lock *sync.Mutex
}

// Creates a new NodeSet service manager
func NewNodeSetServiceManager(sp IHyperdriveServiceProvider) *NodeSetServiceManager {
	wallet := sp.GetWallet()
	resources := sp.GetResources()
	cfg := sp.GetConfig()

	return &NodeSetServiceManager{
		wallet:                 wallet,
		resources:              resources,
		v3Client:               apiv3.NewNodeSetClient(resources.NodeSetApiUrl, time.Duration(cfg.ClientTimeout.Value)*time.Second),
		nodeRegistrationStatus: api.NodeSetRegistrationStatus_Unknown,
		lock:                   &sync.Mutex{},
	}
}

// Get the registration status of the node
func (m *NodeSetServiceManager) GetRegistrationStatus(ctx context.Context) (api.NodeSetRegistrationStatus, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Force refresh the registration status if it hasn't been determined yet
	if m.nodeRegistrationStatus == api.NodeSetRegistrationStatus_Unknown ||
		m.nodeRegistrationStatus == api.NodeSetRegistrationStatus_NoWallet {
		err := m.loginImpl(ctx)
		return m.nodeRegistrationStatus, err
	}
	return m.nodeRegistrationStatus, nil
}

// Log in to the NodeSet server
func (m *NodeSetServiceManager) Login(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.loginImpl(ctx)
}

// Result of RegisterNode
type RegistrationResult int

const (
	RegistrationResult_Unknown RegistrationResult = iota
	RegistrationResult_Success
	RegistrationResult_AlreadyRegistered
	RegistrationResult_NotWhitelisted
)

// Register the node with the NodeSet server
func (m *NodeSetServiceManager) RegisterNode(ctx context.Context, email string) (RegistrationResult, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Registering node with NodeSet")

	// Make sure there's a wallet
	walletStatus, err := m.wallet.GetStatus()
	if err != nil {
		return RegistrationResult_Unknown, fmt.Errorf("error getting wallet status: %w", err)
	}
	if !walletStatus.Wallet.IsLoaded {
		return RegistrationResult_Unknown, fmt.Errorf("can't register node with NodeSet, wallet not loaded")
	}

	// Run the request
	err = m.v3Client.Core.NodeAddress(ctx, logger.Logger, email, walletStatus.Wallet.WalletAddress, m.wallet.SignMessage)
	if err != nil {
		m.setRegistrationStatus(api.NodeSetRegistrationStatus_Unknown)
		if errors.Is(err, core.ErrAlreadyRegistered) {
			return RegistrationResult_AlreadyRegistered, nil
		} else if errors.Is(err, core.ErrNotWhitelisted) {
			return RegistrationResult_NotWhitelisted, nil
		}
		return RegistrationResult_Unknown, fmt.Errorf("error registering node: %w", err)
	}
	return RegistrationResult_Success, nil
}

// =========================
// === StakeWise Methods ===
// =========================

// Get the metadata for the node account with respect to the provided vault
func (m *NodeSetServiceManager) StakeWise_GetValidatorsInfoForNodeAccount(ctx context.Context, deployment string, vault common.Address) (stakewise.ValidatorsMetaData, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Getting server validators info for node account")

	// Run the request
	var data stakewise.ValidatorsMetaData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.StakeWise.ValidatorMeta_Get(ctx, logger.Logger, deployment, vault)
		return err
	})
	if err != nil {
		return stakewise.ValidatorsMetaData{}, fmt.Errorf("error getting validators info for node account: %w", err)
	}
	return data, nil
}

// Send validator deposit info and exit messages to the NodeSet service, and have it sign them for permitting StakeWise deposits
func (m *NodeSetServiceManager) StakeWise_GetValidatorManagerSignature(ctx context.Context, deployment string, vault common.Address, beaconDepositRoot common.Hash, depositData []beacon.ExtendedDepositData, encryptedExitMessages []string) (string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Validation
	if len(depositData) != len(encryptedExitMessages) {
		return "", fmt.Errorf("deposit data and exit messages lengths don't match")
	}
	logger.Debug("Getting validators manager signature")

	// Run the request
	validators := make([]v3stakewise.ValidatorRegistrationDetails, len(depositData))
	for i, data := range depositData {
		validators[i] = v3stakewise.ValidatorRegistrationDetails{
			DepositData: data,
			ExitMessage: encryptedExitMessages[i],
		}
	}
	var data v3stakewise.PostValidatorData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.StakeWise.Validators_Post(ctx, logger.Logger, deployment, vault, validators, beaconDepositRoot)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("error getting validator manager signature: %w", err)
	}
	return data.Signature, nil
}

// Get the vaults for the provided deployment
func (m *NodeSetServiceManager) StakeWise_GetVaults(ctx context.Context, deployment string) ([]v3stakewise.VaultInfo, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Getting registered validators")

	// Run the request
	var data v3stakewise.VaultsData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.StakeWise.Vaults(ctx, logger.Logger, deployment)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error getting registered validators: %w", err)
	}
	return data.Vaults, nil
}

// Get the validators that have been registered on the provided vault
func (m *NodeSetServiceManager) StakeWise_GetRegisteredValidators(ctx context.Context, deployment string, vault common.Address) ([]v3stakewise.ValidatorStatus, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Getting registered validators")

	// Run the request
	var data v3stakewise.ValidatorsData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.StakeWise.Validators_Get(ctx, logger.Logger, deployment, vault)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error getting registered validators: %w", err)
	}
	return data.Validators, nil
}

// =============================
// === Constellation Methods ===
// =============================

// Gets the address that has been registered by the node's user for Constellation.
// Returns nil if the user hasn't registered with NodeSet for Constellation usage yet.
func (m *NodeSetServiceManager) Constellation_GetRegisteredAddress(ctx context.Context, deployment string) (*common.Address, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Getting registered Constellation address")

	// Run the request
	var data v3constellation.Whitelist_GetData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.Constellation.Whitelist_Get(ctx, logger.Logger, deployment)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error getting registered Constellation address: %w", err)
	}
	logger.Debug("NodeSet responded",
		slog.Bool("whitelisted", data.Whitelisted),
		slog.String("address", data.Address.Hex()),
	)

	// Return the address if whitelisted
	if data.Whitelisted {
		return &data.Address, nil
	}
	return nil, nil
}

// Gets a signature for registering / whitelisting the node with the Constellation contracts
func (m *NodeSetServiceManager) Constellation_GetRegistrationSignature(ctx context.Context, deployment string) ([]byte, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}
	logger.Debug("Registering with the Constellation contracts")

	// Run the request
	var data v3constellation.Whitelist_PostData
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.Constellation.Whitelist_Post(ctx, logger.Logger, deployment)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error registering with Constellation: %w", err)
	}

	// Decode the signature
	sig, err := utils.DecodeHex(data.Signature)
	if err != nil {
		return nil, fmt.Errorf("error decoding signature from server: %w", err)
	}
	return sig, nil
}

// Gets the deposit signature for a minipool from the Constellation contracts
func (m *NodeSetServiceManager) Constellation_GetDepositSignature(ctx context.Context, deployment string, minipoolAddress common.Address, salt *big.Int) ([]byte, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Run the request
	var data v3constellation.MinipoolDepositSignatureData
	logger.Debug("Getting minipool deposit signature")
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.Constellation.MinipoolDepositSignature(ctx, logger.Logger, deployment, minipoolAddress, salt)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error getting deposit signature: %w", err)
	}

	// Decode the signature
	sig, err := utils.DecodeHex(data.Signature)
	if err != nil {
		return nil, fmt.Errorf("error decoding signature from server: %w", err)
	}
	return sig, nil
}

// Get the validators that NodeSet has on record for this node
func (m *NodeSetServiceManager) Constellation_GetValidators(ctx context.Context, deployment string) ([]v3constellation.ValidatorStatus, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Run the request
	var data v3constellation.ValidatorsData
	logger.Debug("Getting validators for node")
	err := m.runRequest(ctx, func(ctx context.Context) error {
		var err error
		data, err = m.v3Client.Constellation.Validators_Get(ctx, logger.Logger, deployment)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error getting validators for node: %w", err)
	}
	return data.Validators, nil
}

// Upload signed exit messages for Constellation minipools to the NodeSet service
func (m *NodeSetServiceManager) Constellation_UploadSignedExitMessages(ctx context.Context, deployment string, exitMessages []nscommon.EncryptedExitData) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Run the request
	logger.Debug("Submitting signed exit messages to nodeset")
	err := m.runRequest(ctx, func(ctx context.Context) error {
		return m.v3Client.Constellation.Validators_Patch(ctx, logger.Logger, deployment, exitMessages)
	})
	if err != nil {
		return fmt.Errorf("error submitting signed exit messages: %w", err)
	}
	return nil
}

// ========================
// === Internal Methods ===
// ========================

// Runs a request to the NodeSet server, re-logging in if necessary
func (m *NodeSetServiceManager) runRequest(ctx context.Context, request func(ctx context.Context) error) error {
	// Run the request
	err := request(ctx)
	if err != nil {
		if errors.Is(err, nscommon.ErrInvalidSession) {
			// Session expired so log in again
			err = m.loginImpl(ctx)
			if err != nil {
				return err
			}

			// Re-run the request
			return request(ctx)
		} else {
			return err
		}
	}
	return nil
}

// Implementation for logging in
func (m *NodeSetServiceManager) loginImpl(ctx context.Context) error {
	// Get the logger
	logger, exists := log.FromContext(ctx)
	if !exists {
		panic("context didn't have a logger!")
	}

	// Get the node wallet
	walletStatus, err := m.wallet.GetStatus()
	if err != nil {
		return fmt.Errorf("error getting wallet status for login: %w", err)
	}
	err = CheckIfWalletReady(walletStatus)
	if err != nil {
		m.nodeRegistrationStatus = api.NodeSetRegistrationStatus_NoWallet
		return fmt.Errorf("can't log into nodeset, hyperdrive wallet not initialized yet")
	}

	// Log the login attempt
	logger.Info("Not authenticated with the NodeSet server, logging in")

	// Get the nonce
	nonceData, err := m.v3Client.Core.Nonce(ctx, logger.Logger)
	if err != nil {
		m.setRegistrationStatus(api.NodeSetRegistrationStatus_Unknown)
		return fmt.Errorf("error getting nonce for login: %w", err)
	}
	logger.Debug("Got nonce for login",
		slog.String("nonce", nonceData.Nonce),
	)

	// Create a new session
	m.setSessionToken(nonceData.Token)

	// Attempt a login
	loginData, err := m.v3Client.Core.Login(ctx, logger.Logger, nonceData.Nonce, walletStatus.Wallet.WalletAddress, m.wallet.SignMessage)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotLoaded) {
			m.setRegistrationStatus(api.NodeSetRegistrationStatus_NoWallet)
			return err
		}
		if errors.Is(err, core.ErrUnregisteredNode) {
			m.setRegistrationStatus(api.NodeSetRegistrationStatus_Unregistered)
			return nil
		}
		m.setRegistrationStatus(api.NodeSetRegistrationStatus_Unknown)
		return fmt.Errorf("error logging in: %w", err)
	}

	// Success
	m.setSessionToken(loginData.Token)
	logger.Info("Logged into NodeSet server")
	m.setRegistrationStatus(api.NodeSetRegistrationStatus_Registered)

	return nil
}

// Sets the session token for the client after logging in
func (m *NodeSetServiceManager) setSessionToken(sessionToken string) {
	m.sessionToken = sessionToken
	m.v3Client.SetSessionToken(sessionToken)
}

// Sets the registration status of the node
func (m *NodeSetServiceManager) setRegistrationStatus(status api.NodeSetRegistrationStatus) {
	// Only set to unknown if it hasn't already been figured out
	if status == api.NodeSetRegistrationStatus_Unknown &&
		(m.nodeRegistrationStatus == api.NodeSetRegistrationStatus_Unregistered ||
			m.nodeRegistrationStatus == api.NodeSetRegistrationStatus_Registered) {
		return
	}

	m.nodeRegistrationStatus = status
}
