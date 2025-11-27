//go:build consensus_testing

package engine

import "btc-federation/pkg/consensus/types"

// ForceSetLockedQC directly sets the lockedQC without phase validation.
// UNSAFE: This bypasses HotStuff protocol safety rules.
// Only use for testing state synchronization scenarios.
func (hc *HotStuffConsensus) ForceSetLockedQC(qc *types.QuorumCertificate) {
	hc.lockedQC = qc
	if hc.safetyRules != nil {
		hc.safetyRules.ForceSetLockedQC(qc)
	}
}

// Verify HotStuffConsensus implements EngineTestable at compile time
var _ EngineTestable = (*HotStuffConsensus)(nil)
