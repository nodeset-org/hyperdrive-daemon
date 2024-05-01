package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nodeset-org/hyperdrive-daemon/shared/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nodeset-org/hyperdrive-daemon/shared"
	"github.com/nodeset-org/hyperdrive-daemon/shared/types"
	"github.com/rocket-pool/node-manager-core/beacon"
	"github.com/rocket-pool/node-manager-core/node/validator"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
)

const (
	DepositAmount uint64 = 32e9 //32 eth?
)

// DepositDataManager manages the aggregated deposit data file that Constellation uses
type DepositDataManager struct {
	dataPath string
	sp       *ServiceProvider
}

// Creates a new manager
func NewDepositDataManager(sp *ServiceProvider) (*DepositDataManager, error) {
	// Is this the correct directory?!
	dataPath := filepath.Join(sp.GetConfig().UserDataPath.Value, config.DepositDataFile)

	ddMgr := &DepositDataManager{
		dataPath: filepath.Join(sp.GetConfig().UserDataPath.Value, config.DepositDataFile),
		sp:       sp,
	}

	// Initialize the file if it's not there
	_, err := os.Stat(dataPath)
	if errors.Is(err, fs.ErrNotExist) {
		// Make a blank one
		err = ddMgr.UpdateDepositData([]types.ExtendedDepositData{})
		return ddMgr, err
	}
	if err != nil {
		return nil, fmt.Errorf("error checking status of wallet file [%s]: %w", dataPath, err)
	}

	return ddMgr, nil
}

// Generates deposit data for the provided keys
func (m *DepositDataManager) GenerateDepositData(keys []*eth2types.BLSPrivateKey, minipool common.Address) ([]*types.ExtendedDepositData, error) {
	resources := m.sp.GetNetworkResources()

	if minipool.Hex() == "" {
		return nil, fmt.Errorf("minipool address is empty")
	}

	// Stakewise uses the same withdrawal creds for each validator
	withdrawalCreds := validator.GetWithdrawalCredsFromAddress(minipool)

	// Create the new aggregated deposit data for all generated keys
	dataList := make([]*types.ExtendedDepositData, len(keys))
	for i, key := range keys {
		depositData, err := validator.GetDepositData(key, withdrawalCreds, resources.GenesisForkVersion, DepositAmount, resources.EthNetworkName)
		if err != nil {
			pubkey := beacon.ValidatorPubkey(key.PublicKey().Marshal())
			return nil, fmt.Errorf("error getting deposit data for key %s: %w", pubkey.HexWithPrefix(), err)
		}
		dataList[i] = &types.ExtendedDepositData{
			ExtendedDepositData: depositData,
			HyperdriveVersion:   shared.HyperdriveVersion,
		}
	}
	return dataList, nil
}

// Save the deposit data file
func (m *DepositDataManager) UpdateDepositData(data []types.ExtendedDepositData) error {
	// Serialize it
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializing deposit data: %w", err)
	}

	// Write it
	err = os.WriteFile(m.dataPath, bytes, fileMode)
	if err != nil {
		return fmt.Errorf("error saving deposit data to disk: %w", err)
	}

	return nil
}
