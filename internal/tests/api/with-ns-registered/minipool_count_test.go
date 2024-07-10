package with_ns_registered

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	hdtesting "github.com/nodeset-org/hyperdrive-daemon/testing"
	"github.com/stretchr/testify/require"
)

const (
	nodeWalletAddressString string = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	goodPassword            string = "some_password123"
)

var (
	nodeWalletAddress common.Address = common.HexToAddress(nodeWalletAddressString)
)

func TestGetMinipoolCount(t *testing.T) {
	// Take a snapshot, revert at the end
	snapshotName, err := testMgr.CreateCustomSnapshot(hdtesting.Service_EthClients | hdtesting.Service_Filesystem | hdtesting.Service_NodeSet)
	if err != nil {
		fail("Error creating custom snapshot: %v", err)
	}
	defer nodeset_cleanup(snapshotName)

	// Set the minipool count
	nsmock := testMgr.GetNodeSetMockServer()
	nsmock.GetManager().SetAvailableConstellationMinipoolCount(nodeWalletAddress, 10)

	// Get the minipool count and assert
	minipoolCountResponse, err := testMgr.GetApiClient().NodeSet_Constellation.GetAvailableMinipoolCount()
	require.NoError(t, err)
	require.Equal(t, 10, minipoolCountResponse.Data.Count)

	t.Log("GetAvailableMinipoolCount called")

}
