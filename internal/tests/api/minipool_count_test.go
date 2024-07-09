package api_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	hdtesting "github.com/nodeset-org/hyperdrive-daemon/testing"
	"github.com/stretchr/testify/require"
)

func TestGetMinipoolCount(t *testing.T) {
	// Take a snapshot, revert at the end
	snapshotName, err := testMgr.CreateCustomSnapshot(hdtesting.Service_Filesystem)
	if err != nil {
		fail("Error creating custom snapshot: %v", err)
	}
	defer wallet_cleanup(snapshotName)

	nsMgr := testMgr.GetNodeSetMockServer().GetManager()
	response, err := nsMgr.GetAvailableConstellationMinipoolCount(common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"))

	require.NoError(t, err)
	require.Equal(t, 10, response)
	t.Log("GetAvailableMinipoolCount called")

}
