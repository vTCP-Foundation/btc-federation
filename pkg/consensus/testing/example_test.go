// Package testing provides example tests demonstrating the event-based testing infrastructure.
package testing

import (
	"testing"
	"time"

	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/mocks"
	"btc-federation/pkg/consensus/types"
)

// TestEventTracerBasicFunctionality tests the basic event tracer functionality
func TestEventTracerBasicFunctionality(t *testing.T) {
	tracer := mocks.NewConsensusEventTracer()
	
	// Record some events
	tracer.RecordEvent(0, events.EventProposalCreated, events.EventPayload{
		"view": types.ViewNumber(1),
		"block_hash": "test_hash",
	})
	
	tracer.RecordEvent(1, events.EventVoteCreated, events.EventPayload{
		"view": types.ViewNumber(1),
		"phase": types.PhasePrepare,
	})
	
	tracer.RecordTransition(0, events.StateIdle, events.StateProposing, "proposal_trigger")
	
	// Validate events were recorded
	allEvents := tracer.GetEvents()
	if len(allEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(allEvents))
	}
	
	// Test filtering by type
	proposalEvents := tracer.GetEventsByType(events.EventProposalCreated)
	if len(proposalEvents) != 1 {
		t.Errorf("Expected 1 proposal event, got %d", len(proposalEvents))
	}
	
	// Test filtering by node
	node0Events := tracer.GetEventsByNode(0)
	if len(node0Events) != 2 {
		t.Errorf("Expected 2 events for node 0, got %d", len(node0Events))
	}
	
	// Test filtering by node and type
	node0Proposals := tracer.GetEventsByNodeAndType(0, events.EventProposalCreated)
	if len(node0Proposals) != 1 {
		t.Errorf("Expected 1 proposal event for node 0, got %d", len(node0Proposals))
	}
}

// TestValidationRules tests the validation rule system
func TestValidationRules(t *testing.T) {
	tracer := mocks.NewConsensusEventTracer()
	
	// Create events that should pass validation
	events := []events.ConsensusEvent{
		{
			NodeID:    0,
			EventType: events.EventProposalCreated,
			Timestamp: time.Now(),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventProposalReceived,
			Timestamp: time.Now().Add(10 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventVoteSent,
			Timestamp: time.Now().Add(20 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
	}
	
	// Test MustHappenBeforeRule
	rule := &MustHappenBeforeRule{
		EventA:      events.EventProposalReceived,
		EventB:      events.EventVoteSent,
		Description: "Proposal must be received before voting",
	}
	
	errors := rule.Validate(events)
	if len(errors) != 0 {
		t.Errorf("Expected no validation errors, got %d: %v", len(errors), errors)
	}
	
	// Test with invalid order (should fail)
	invalidEvents := []events.ConsensusEvent{
		{
			NodeID:    1,
			EventType: events.EventVoteSent,
			Timestamp: time.Now(),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventProposalReceived,
			Timestamp: time.Now().Add(10 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
	}
	
	errors = rule.Validate(invalidEvents)
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid event order, got none")
	}
}

// TestQuorumRule tests the quorum validation rule
func TestQuorumRule(t *testing.T) {
	// Create events where only 2 nodes vote (insufficient for f=1, need 3)
	events := []events.ConsensusEvent{
		{
			NodeID:    1,
			EventType: events.EventVoteSent,
			Timestamp: time.Now(),
		},
		{
			NodeID:    2,
			EventType: events.EventVoteSent,
			Timestamp: time.Now(),
		},
	}
	
	rule := &QuorumRule{
		EventType:   events.EventVoteSent,
		MinNodes:    3, // Requires 3 votes for quorum
		Description: "Quorum of nodes must vote",
	}
	
	errors := rule.Validate(events)
	if len(errors) != 1 {
		t.Errorf("Expected 1 validation error, got %d", len(errors))
	}
	
	// Add third vote to make it valid
	events = append(events, events.ConsensusEvent{
		NodeID:    3,
		EventType: events.EventVoteSent,
		Timestamp: time.Now(),
	})
	
	errors = rule.Validate(events)
	if len(errors) != 0 {
		t.Errorf("Expected no validation errors with quorum, got %d", len(errors))
	}
}

// TestLeaderOnlyRule tests that certain events only occur on leader nodes
func TestLeaderOnlyRule(t *testing.T) {
	// Node 0 is leader for view 0, node 1 for view 1, etc.
	validEvents := []events.ConsensusEvent{
		{
			NodeID:    0,
			EventType: events.EventProposalCreated,
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventProposalCreated,
			Payload: events.EventPayload{
				"view": types.ViewNumber(1),
			},
		},
	}
	
	rule := &LeaderOnlyRule{
		EventType:   events.EventProposalCreated,
		Description: "Only leader should create proposals",
	}
	
	errors := rule.Validate(validEvents)
	if len(errors) != 0 {
		t.Errorf("Expected no validation errors, got %d", len(errors))
	}
	
	// Test invalid case - non-leader creating proposal
	invalidEvents := []events.ConsensusEvent{
		{
			NodeID:    1, // Node 1 is not leader for view 0
			EventType: events.EventProposalCreated,
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
	}
	
	errors = rule.Validate(invalidEvents)
	if len(errors) != 1 {
		t.Errorf("Expected 1 validation error, got %d", len(errors))
	}
}

// TestHappyPathRules tests the complete happy path rule set
func TestHappyPathRules(t *testing.T) {
	// Create a sequence of events representing a successful consensus round
	now := time.Now()
	events := []events.ConsensusEvent{
		// Leader creates proposal
		{
			NodeID:    0,
			EventType: events.EventProposalCreated,
			Timestamp: now,
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		// Validators receive and validate proposal
		{
			NodeID:    1,
			EventType: events.EventProposalReceived,
			Timestamp: now.Add(10 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventProposalValidated,
			Timestamp: now.Add(15 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventVoteCreated,
			Timestamp: now.Add(20 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    1,
			EventType: events.EventVoteSent,
			Timestamp: now.Add(25 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		// Additional validators vote
		{
			NodeID:    2,
			EventType: events.EventVoteSent,
			Timestamp: now.Add(30 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		{
			NodeID:    3,
			EventType: events.EventVoteSent,
			Timestamp: now.Add(35 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		// QC formation and block commitment
		{
			NodeID:    0,
			EventType: events.EventQCFormed,
			Timestamp: now.Add(40 * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(0),
			},
		},
		// All nodes commit block
		{
			NodeID:    0,
			EventType: events.EventBlockCommitted,
			Timestamp: now.Add(50 * time.Millisecond),
		},
		{
			NodeID:    1,
			EventType: events.EventBlockCommitted,
			Timestamp: now.Add(52 * time.Millisecond),
		},
		{
			NodeID:    2,
			EventType: events.EventBlockCommitted,
			Timestamp: now.Add(54 * time.Millisecond),
		},
		{
			NodeID:    3,
			EventType: events.EventBlockCommitted,
			Timestamp: now.Add(56 * time.Millisecond),
		},
	}
	
	// Test against happy path rules
	rules := GetHappyPathRules(4, 1) // 4 nodes, 1 Byzantine tolerated
	errors := ValidateEvents(events, rules)
	
	if len(errors) != 0 {
		t.Errorf("Expected no validation errors for happy path, got %d errors:", len(errors))
		for _, err := range errors {
			t.Errorf("  - %v", err)
		}
	}
}

// TestNoOpEventTracer tests that the no-op tracer has zero overhead
func TestNoOpEventTracer(t *testing.T) {
	tracer := &mocks.NoOpEventTracer{}
	
	// These should all be no-ops
	tracer.RecordEvent(0, events.EventProposalCreated, events.EventPayload{})
	tracer.RecordTransition(0, events.StateIdle, events.StateProposing, "test")
	tracer.RecordMessage(0, events.MessageOutbound, "TestMsg", events.EventPayload{})
	
	// No way to verify they did nothing, but they should not panic or error
	// This test mainly exists to ensure the no-op methods compile and run
}

// BenchmarkEventTracer benchmarks event recording performance
func BenchmarkEventTracer(b *testing.B) {
	tracer := mocks.NewConsensusEventTracer()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.RecordEvent(uint16(i%5), events.EventProposalCreated, events.EventPayload{
			"view": types.ViewNumber(i),
			"round": i,
		})
	}
}

// BenchmarkValidation benchmarks rule validation performance
func BenchmarkValidation(b *testing.B) {
	// Create a set of events to validate against
	events := make([]events.ConsensusEvent, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = events.ConsensusEvent{
			NodeID:    uint16(i % 5),
			EventType: events.EventVoteSent,
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			Payload: events.EventPayload{
				"view": types.ViewNumber(i / 10),
			},
		}
	}
	
	rules := GetHappyPathRules(5, 1)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateEvents(events, rules)
	}
}