package with_ns_registered

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetMinipoolCount(t *testing.T) {
	// Take a snapshot, revert at the end
	// snapshotName, err := testMgr.CreateCustomSnapshot(hdtesting.Service_Filesystem)
	// if err != nil {
	// 	fail("Error creating custom snapshot: %v", err)
	// }
	// defer wallet_cleanup(snapshotName)
	response, err := testMgr.GetApiClient().NodeSet_Constellation.GetAvailableMinipoolCount()
	require.NoError(t, err)
	require.Equal(t, 10, response.Data.Count)
	t.Log("GetAvailableMinipoolCount called")

	// // Run the round-trip test
	// derivationPath := string(wallet.DerivationPath_Default)
	// index := uint64(0)
	// response, err := testMgr.GetApiClient().Wallet.Recover(&derivationPath, keys.DefaultMnemonic, &index, goodPassword, true)
	// require.NoError(t, err)
	// t.Log("Recover called")

	// // Check the response
	// require.Equal(t, expectedWalletAddress, response.Data.AccountAddress)
	// t.Log("Received correct wallet address")
}
