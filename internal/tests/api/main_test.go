package api_test

import (
	"fmt"
	"log/slog"
	"os"
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
