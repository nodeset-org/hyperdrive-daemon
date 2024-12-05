package api_test

import (
	"testing"

	"github.com/nodeset-org/hyperdrive-daemon/shared/types/api"
	"github.com/nodeset-org/osha/keys"
	"github.com/rocket-pool/node-manager-core/wallet"
	"github.com/stretchr/testify/require"
)

const (
	nsEmail string = "test@nodeset.io"
)

var nodesetTestWalletRecoveredSnapshot string

// Test registration with nodeset.io if the node doesn't have a wallet yet
func TestNodeSetRegistration_NoWallet(t *testing.T) {
	err := testMgr.DependsOnBaseline()
	require.NoError(t, err)

	// Run the round-trip test
	hd := hdNode.GetApiClient()
	response, err := hd.NodeSet.GetRegistrationStatus()
	require.NoError(t, err)
	require.Equal(t, api.NodeSetRegistrationStatus_NoWallet, response.Data.Status)
	t.Logf("Node has no wallet, registration status is correct")
}

// Test registration with nodeset.io if the node has a wallet but hasn't been registered yet
func TestNodeSetRegistration_NoRegistration(t *testing.T) {
	err := testMgr.DependsOnBaseline()
	require.NoError(t, err)

	// Recover a wallet
	derivationPath := string(wallet.DerivationPath_Default)
	index := uint64(0)
	recoverResponse, err := hdNode.GetApiClient().Wallet.Recover(&derivationPath, keys.DefaultMnemonic, &index, goodPassword, true)
	require.NoError(t, err)
	t.Log("Recover called")

	nodesetTestWalletRecoveredSnapshot, err = testMgr.CreateSnapshot()
	if err != nil {
		fail("Error creating custom snapshot: %v", err)
	}

	// Check the response
	require.Equal(t, expectedWalletAddress, recoverResponse.Data.AccountAddress)
	t.Log("Received correct wallet address")

	// Run the round-trip test
	hd := hdNode.GetApiClient()
	registrationResponse, err := hd.NodeSet.GetRegistrationStatus()
	require.NoError(t, err)
	require.Equal(t, api.NodeSetRegistrationStatus_Unregistered, registrationResponse.Data.Status)
	t.Logf("Node has a wallet but isn't registered, registration status is correct")
}

// Test registration with nodeset.io if the node has a wallet and has been registered
func TestNodeSetRegistration_Registered(t *testing.T) {
	// Recover wallet loaded snapshot, revert at the end
	err := testMgr.DependsOn(TestNodeSetRegistration_NoRegistration, &nodesetTestWalletRecoveredSnapshot, t)
	require.NoError(t, err)

	// Check the response
	apiClient := hdNode.GetApiClient()
	response, err := apiClient.Wallet.Status()
	require.NoError(t, err)
	require.Equal(t, expectedWalletAddress, response.Data.WalletStatus.Address.NodeAddress)
	t.Log("Received correct wallet address")
	// Register the node with nodeset.io
	hd := hdNode.GetApiClient()
	nsMgr := testMgr.GetNodeSetMockServer().GetManager()
	nsDB := nsMgr.GetDatabase()
	user, err := nsDB.Core.AddUser(nsEmail)
	require.NoError(t, err)
	_ = user.WhitelistNode(expectedWalletAddress)
	require.NoError(t, err)
	registerResponse, err := hd.NodeSet.RegisterNode(nsEmail)
	require.NoError(t, err)
	require.True(t, registerResponse.Data.Success)

	// Run the round-trip test
	registrationResponse, err := hd.NodeSet.GetRegistrationStatus()
	require.NoError(t, err)
	require.Equal(t, api.NodeSetRegistrationStatus_Registered, registrationResponse.Data.Status)
	t.Logf("Node is registered with nodeset.io")

}
