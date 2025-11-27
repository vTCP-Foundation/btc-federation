//go:build consensus_testing

package integration

import "btc-federation/pkg/consensus/types"

// StateRecoverable provides test-only state recovery capabilities.
// This interface is implemented by HotStuffCoordinator only when built
// with the "testing" tag (see coordinator_testing.go).
//
// This allows the mock package to perform state synchronization without
// exposing internal coordinator fields, while ensuring the recovery method
// is never available in production builds.
type StateRecoverable interface {
	// ForceRecoverState updates coordinator state to simulate state sync recovery.
	// It sets the current view, updates highestQC and lockedQC, clears stale vote
	// collections, and restarts the view timer.
	//
	// This method is only available in test builds (//go:build testing).
	ForceRecoverState(
		targetView types.ViewNumber,
		highestQC *types.QuorumCertificate,
		lockedQC *types.QuorumCertificate,
	) error
}
