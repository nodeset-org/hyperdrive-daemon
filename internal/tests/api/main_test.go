package api_test

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"testing"

	hdtesting "github.com/nodeset-org/hyperdrive-daemon/testing"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/log"
)

// Various singleton variables used for testing
var (
	testMgr *hdtesting.HyperdriveTestManager = nil
	wg      *sync.WaitGroup                  = nil
	logger  *slog.Logger                     = nil
	hdNode  *hdtesting.HyperdriveNode
)

var baseSnapshot string

// Initialize a common server used by all tests
func TestMain(m *testing.M) {
	wg = &sync.WaitGroup{}
	var err error
	testMgr, err = hdtesting.NewHyperdriveTestManagerWithDefaults(func(ns *config.NetworkSettings) *config.NetworkSettings {
		return ns
	})
	if err != nil {
		fail("error creating test manager: %v", err)
	}

	logger = testMgr.GetLogger()
	hdNode = testMgr.GetNode()

	baseSnapshot, err = testMgr.CreateSnapshot()
	if err != nil {
		fail("Error creating base snapshot: %v", err)
	}

	// Run tests
	code := m.Run()

	// Clean up and exit
	cleanup()
	os.Exit(code)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	cleanup()
	os.Exit(1)
}

func cleanup() {
	if testMgr == nil {
		return
	}
	err := testMgr.Close()
	if err != nil {
		logger.Error("Error closing test manager", log.Err(err))
	}
	testMgr = nil
}

// Clean up after each test
func test_cleanup(t *testing.T) {
	// Handle panics
	r := recover()
	if r != nil {
		debug.PrintStack()
		fail("Recovered from panic: %v", r)
	}

	if baseSnapshot == "" {
		fail("Base snapshot is not defined or is empty")
	}

	// Revert to the snapshot taken at the start of the test
	err := testMgr.RevertSnapshot(baseSnapshot)
	if err != nil {
		fail("Error reverting to custom snapshot: %v", err)
	}

	// Reload the wallet to undo any changes made during the test
	err = hdNode.GetServiceProvider().GetWallet().Reload(testMgr.GetLogger())
	if err != nil {
		fail("Error reloading wallet: %v", err)
	}
}
