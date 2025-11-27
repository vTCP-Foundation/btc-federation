//go:build consensus_testing

package engine

import "btc-federation/pkg/consensus/types"

// EngineTestable provides DANGEROUS test-only state manipulation for HotStuffConsensus.
// This interface is implemented by HotStuffConsensus only when built
// with the "consensus_testing" tag (see consensus_testing.go).
//
// WARNING: These methods bypass HotStuff protocol safety rules and must
// NEVER be used in production code. They exist solely for testing state
// synchronization scenarios where we need to set lockedQC without
// phase validation.
type EngineTestable interface {
	// ForceSetLockedQC directly sets the lockedQC without phase validation.
	// UNSAFE: This bypasses HotStuff protocol safety rules.
	// Only use for testing state synchronization scenarios.
	ForceSetLockedQC(qc *types.QuorumCertificate)
}
