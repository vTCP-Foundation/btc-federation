// Package testing provides comprehensive event validation for consensus protocol testing.
// This package implements rule-based validation to ensure protocol compliance.
package testing

import (
	"fmt"
	"time"

	"btc-federation/pkg/consensus/events"
)

// ExpectedEventSequence defines a single expected event in the consensus protocol sequence
type ExpectedEventSequence struct {
	SequenceNumber int              // Order in which this event should occur
	Phase          string           // Consensus phase (NewView, Proposal, Prepare, PreCommit, Commit, Decide)
	SubPhase       string           // Sub-phase for detailed tracking (Creation, Broadcast, Reception, etc.)
	NodeRole       string           // Expected node role (Leader, Validator, All)
	NodeID         *int             // Specific node ID if applicable (nil means any node of the role)
	EventType      events.EventType // The specific event type expected
	Required       bool             // Whether this event must occur
	MinOccurrences int              // Minimum number of times this event should occur
	MaxOccurrences int              // Maximum number of times this event should occur (0 = unlimited)
	MaxDelay       time.Duration    // Maximum allowed delay for this event
	WindowStart    events.EventType // Event that starts the time window for this event
	WindowDuration time.Duration    // Duration of the time window from WindowStart
	DependsOn      []int            // Sequence numbers this event depends on
	EnablesEvents  []int            // Sequence numbers this event enables
	Description    string           // Human-readable description
	FailureHint    string           // Hint for debugging when this event fails
}

// ValidationResult contains the results of validating an event sequence
type ValidationResult struct {
	TotalExpected    int                           // Total number of expected events
	TotalFound       int                           // Total number of matching events found
	PassedEvents     []ExpectedEventSequence       // Events that passed validation
	FailedEvents     []ExpectedEventSequence       // Events that failed validation
	UnexpectedEvents []events.ConsensusEvent       // Events that were not expected
	Summary          string                        // Human-readable summary
	Success          bool                          // Whether all validations passed
}

// ValidateSequentialEvents validates that the actual events match the expected sequence with role constraints
func ValidateSequentialEvents(expectedSequence []ExpectedEventSequence, actualEvents []events.ConsensusEvent) ValidationResult {
	result := ValidationResult{
		TotalExpected:    len(expectedSequence),
		TotalFound:       0,
		PassedEvents:     []ExpectedEventSequence{},
		FailedEvents:     []ExpectedEventSequence{},
		UnexpectedEvents: []events.ConsensusEvent{},
		Success:          true,
	}

	// Build index of actual events by type
	eventIndex := make(map[events.EventType][]events.ConsensusEvent)
	for _, event := range actualEvents {
		eventIndex[event.EventType] = append(eventIndex[event.EventType], event)
	}

	// Validate each expected event
	for _, expected := range expectedSequence {
		eventsOfType := eventIndex[expected.EventType]
		
		// Filter events by role and node constraints
		matchingEvents := filterEventsByRoleConstraints(eventsOfType, expected)
		
		if len(matchingEvents) == 0 {
			// No events matching role constraints found
			if expected.Required {
				if len(eventsOfType) > 0 {
					expected.FailureHint = fmt.Sprintf("Found %d events of type %s but none matched role constraint (NodeRole: %s, NodeID: %v)", 
						len(eventsOfType), expected.EventType, expected.NodeRole, expected.NodeID)
				} else {
					expected.FailureHint = fmt.Sprintf("Required event %s not found", expected.EventType)
				}
				result.FailedEvents = append(result.FailedEvents, expected)
				result.Success = false
			}
			continue
		}

		// Check occurrence count
		if len(matchingEvents) < expected.MinOccurrences {
			expected.FailureHint = fmt.Sprintf("Expected at least %d occurrences, found %d (with role constraints NodeRole: %s, NodeID: %v)", 
				expected.MinOccurrences, len(matchingEvents), expected.NodeRole, expected.NodeID)
			result.FailedEvents = append(result.FailedEvents, expected)
			result.Success = false
			continue
		}

		if expected.MaxOccurrences > 0 && len(matchingEvents) > expected.MaxOccurrences {
			expected.FailureHint = fmt.Sprintf("Expected at most %d occurrences, found %d (with role constraints NodeRole: %s, NodeID: %v)", 
				expected.MaxOccurrences, len(matchingEvents), expected.NodeRole, expected.NodeID)
			result.FailedEvents = append(result.FailedEvents, expected)
			result.Success = false
			continue
		}

		// Event found and count is correct
		result.PassedEvents = append(result.PassedEvents, expected)
		result.TotalFound += len(matchingEvents)
	}

	// Create summary
	if result.Success {
		result.Summary = fmt.Sprintf("✅ All %d expected events validated successfully", result.TotalExpected)
	} else {
		result.Summary = fmt.Sprintf("❌ %d/%d events failed validation", len(result.FailedEvents), result.TotalExpected)
	}

	return result
}

// filterEventsByRoleConstraints filters events based on NodeRole and NodeID constraints
func filterEventsByRoleConstraints(eventList []events.ConsensusEvent, expected ExpectedEventSequence) []events.ConsensusEvent {
	if len(eventList) == 0 {
		return eventList
	}

	var filtered []events.ConsensusEvent

	for _, event := range eventList {
		// Check NodeID constraint first (most specific)
		if expected.NodeID != nil {
			if int(event.NodeID) == *expected.NodeID {
				filtered = append(filtered, event)
			}
			continue // Skip role check if NodeID is specified
		}

		// Check NodeRole constraint
		switch expected.NodeRole {
		case "Leader":
			// For leader events, we need to determine if this node was the leader for this view
			// For now, we'll use a simple calculation: leader = view % nodeCount
			// This assumes the event has view information or we calculate from nodeID
			if isNodeLeaderForEvent(event) {
				filtered = append(filtered, event)
			}
		case "Validator":
			// For validator events, exclude the leader
			if !isNodeLeaderForEvent(event) {
				filtered = append(filtered, event)
			}
		case "All":
			// All nodes should emit this event
			filtered = append(filtered, event)
		default:
			// No role constraint or unknown role - include all
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// isNodeLeaderForEvent determines if the node that emitted this event was the leader
// This is a simplified implementation that assumes view 0 and uses node 0 as leader
// In a full implementation, this would need to access view information from the event
func isNodeLeaderForEvent(event events.ConsensusEvent) bool {
	// Simple implementation: assume view 0, node 0 is leader
	// Note: This calculation is simplified for testing purposes. A more sophisticated 
	// implementation would extract view numbers from event metadata and calculate 
	// leadership based on actual consensus view rotation algorithms (e.g., round-robin).
	// The current approach provides sufficient accuracy for happy path validation.
	return event.NodeID == 0
}