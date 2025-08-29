// Package testing provides comprehensive event validation for consensus protocol testing.
// This package implements rule-based validation to ensure protocol compliance.
package testing

import (
	"fmt"
	"time"

	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/types"
)

// ValidationRule defines an interface for validating consensus event sequences.
// Each rule can examine the complete event history and report violations.
type ValidationRule interface {
	// Validate examines events and returns any violations found
	Validate(eventList []events.ConsensusEvent) []ValidationError
	
	// Description returns a human-readable description of what this rule validates
	Description() string
}

// ValidationError represents a rule violation found during event validation
type ValidationError struct {
	Rule        string                     `json:"rule"`
	RuleDescription string                     `json:"description"`
	Events      []events.ConsensusEvent    `json:"events,omitempty"`
	Severity    Severity                    `json:"severity"`
	NodeID      uint16                      `json:"node_id,omitempty"`
	View        uint64                      `json:"view,omitempty"`
}

// Error implements the error interface
func (ve ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", ve.Severity, ve.Rule, ve.RuleDescription)
}

// Severity indicates how critical a validation error is
type Severity string

const (
	SeverityError   Severity = "error"   // Protocol violation - must not happen
	SeverityWarning Severity = "warning" // Unusual but allowed behavior
	SeverityInfo    Severity = "info"    // Informational observation
)

// MustHappenBeforeRule validates that event A must occur before event B
type MustHappenBeforeRule struct {
	EventA      events.EventType `json:"event_a"`
	EventB      events.EventType `json:"event_b"`
	RuleDescription string        `json:"description"`
}

// Validate checks that EventA happens before EventB for each node
func (r *MustHappenBeforeRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	nodeEvents := groupEventsByNode(eventList)
	
	for nodeID, nodeEventList := range nodeEvents {
		eventAIndex := -1
		eventBIndex := -1
		
		for i, event := range nodeEventList {
			if event.EventType == r.EventA {
				eventAIndex = i
			}
			if event.EventType == r.EventB {
				if eventBIndex == -1 { // Only record first occurrence
					eventBIndex = i
				}
			}
		}
		
		// If EventB occurred but EventA didn't, or EventA occurred after EventB
		if eventBIndex != -1 && (eventAIndex == -1 || eventAIndex > eventBIndex) {
			errors = append(errors, ValidationError{
				Rule:        r.RuleDescription,
				RuleDescription: fmt.Sprintf("Node %d: %s must happen before %s", 
					nodeID, r.EventA, r.EventB),
				Severity:    SeverityError,
				NodeID:      nodeID,
			})
		}
	}
	
	return errors
}

// Description returns rule description
func (r *MustHappenBeforeRule) Description() string {
	return r.RuleDescription
}

// MustNotHappenRule validates that certain events should never occur under specific conditions
type MustNotHappenRule struct {
	EventType   events.EventType                         `json:"event_type"`
	Condition   func(event events.ConsensusEvent) bool   `json:"-"` // Not serializable
	RuleDescription string                               `json:"description"`
}

// Validate checks that forbidden events don't occur
func (r *MustNotHappenRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	
	for _, event := range eventList {
		if event.EventType == r.EventType && (r.Condition == nil || r.Condition(event)) {
			errors = append(errors, ValidationError{
				Rule:        r.RuleDescription,
				RuleDescription: fmt.Sprintf("Forbidden event occurred: %s on node %d", 
					r.EventType, event.NodeID),
				Events:      []events.ConsensusEvent{event},
				Severity:    SeverityError,
				NodeID:      event.NodeID,
			})
		}
	}
	
	return errors
}

// Description returns rule description
func (r *MustNotHappenRule) Description() string {
	return r.RuleDescription
}

// QuorumRule validates that sufficient nodes participate in consensus events
type QuorumRule struct {
	EventType     events.EventType `json:"event_type"`
	MinNodes      uint16               `json:"min_nodes"`
	RuleDescription   string           `json:"description"`
}

// Validate checks that enough nodes participate in the event
func (r *QuorumRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	nodesWithEvent := make(map[uint16]bool)
	
	for _, event := range eventList {
		if event.EventType == r.EventType {
			nodesWithEvent[event.NodeID] = true
		}
	}
	
	if uint16(len(nodesWithEvent)) < r.MinNodes {
		return []ValidationError{{
			Rule:        r.RuleDescription,
			RuleDescription: fmt.Sprintf("Only %d nodes had %s, minimum %d required", 
				len(nodesWithEvent), r.EventType, r.MinNodes),
			Severity:    SeverityError,
		}}
	}
	
	return []ValidationError{}
}

// Description returns rule description
func (r *QuorumRule) Description() string {
	return r.RuleDescription
}

// LeaderOnlyRule validates that certain events only occur on leader nodes
type LeaderOnlyRule struct {
	EventType   events.EventType `json:"event_type"`
	RuleDescription string               `json:"description"`
}

// Validate checks that events only occur on designated leaders
func (r *LeaderOnlyRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	
	for _, event := range eventList {
		if event.EventType == r.EventType {
			view := getViewFromEvent(event)
			expectedLeader := calculateLeader(view, getTotalNodes())
			
			if event.NodeID != expectedLeader {
				errors = append(errors, ValidationError{
					Rule:        r.RuleDescription,
					RuleDescription: fmt.Sprintf("Event %s occurred on node %d but leader for view %d should be node %d", 
						r.EventType, event.NodeID, view, expectedLeader),
					Events:      []events.ConsensusEvent{event},
					Severity:    SeverityError,
					NodeID:      event.NodeID,
					View:        view,
				})
			}
		}
	}
	
	return errors
}

// Description returns rule description
func (r *LeaderOnlyRule) Description() string {
	return r.RuleDescription
}

// ValidatorOnlyRule validates that certain events only occur on validator (non-leader) nodes
type ValidatorOnlyRule struct {
	EventType   events.EventType `json:"event_type"`
	RuleDescription string               `json:"description"`
}

// Validate checks that events only occur on validators, not leaders
func (r *ValidatorOnlyRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	
	for _, event := range eventList {
		if event.EventType == r.EventType {
			view := getViewFromEvent(event)
			expectedLeader := calculateLeader(view, getTotalNodes())
			
			if event.NodeID == expectedLeader {
				errors = append(errors, ValidationError{
					Rule:        r.RuleDescription,
					RuleDescription: fmt.Sprintf("Event %s occurred on leader node %d but should only occur on validators", 
						r.EventType, event.NodeID),
					Events:      []events.ConsensusEvent{event},
					Severity:    SeverityError,
					NodeID:      event.NodeID,
					View:        view,
				})
			}
		}
	}
	
	return errors
}

// Description returns rule description
func (r *ValidatorOnlyRule) Description() string {
	return r.RuleDescription
}

// CausalDependencyRule validates that trigger events have required prerequisite events
type CausalDependencyRule struct {
	TriggerEvent   events.EventType `json:"trigger_event"`
	RequiredEvent  events.EventType `json:"required_event"`
	TimeWindow     time.Duration        `json:"time_window"`
	RuleDescription    string               `json:"description"`
}

// Validate checks that trigger events have required causal dependencies
func (r *CausalDependencyRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	nodeEvents := groupEventsByNode(eventList)
	
	for nodeID, nodeEventList := range nodeEvents {
		for i, event := range nodeEventList {
			if event.EventType == r.TriggerEvent {
				found := false
				
				// Look for required event within time window before trigger
				for j := i - 1; j >= 0; j-- {
					prevEvent := nodeEventList[j]
					if prevEvent.EventType == r.RequiredEvent {
						if event.Timestamp.Sub(prevEvent.Timestamp) <= r.TimeWindow {
							found = true
							break
						}
					}
				}
				
				if !found {
					errors = append(errors, ValidationError{
						Rule:        r.RuleDescription,
						RuleDescription: fmt.Sprintf("Node %d: %s occurred without %s within %v", 
							nodeID, r.TriggerEvent, r.RequiredEvent, r.TimeWindow),
						Events:      []events.ConsensusEvent{event},
						Severity:    SeverityError,
						NodeID:      nodeID,
					})
				}
			}
		}
	}
	
	return errors
}

// Description returns rule description
func (r *CausalDependencyRule) Description() string {
	return r.RuleDescription
}

// CrossNodeCoordinationRule validates coordination between multiple nodes
type CrossNodeCoordinationRule struct {
	InitiatorEvent events.EventType `json:"initiator_event"`
	ResponseEvent  events.EventType `json:"response_event"`
	MinResponders  uint16               `json:"min_responders"`
	MaxDelay       time.Duration        `json:"max_delay"`
	RuleDescription    string               `json:"description"`
}

// Validate checks that initiator events receive sufficient responses
func (r *CrossNodeCoordinationRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	
	// Find all initiator events
	initiatorEvents := []events.ConsensusEvent{}
	for _, event := range eventList {
		if event.EventType == r.InitiatorEvent {
			initiatorEvents = append(initiatorEvents, event)
		}
	}
	
	// For each initiator, count valid responses
	for _, initEvent := range initiatorEvents {
		validResponses := 0
		cutoffTime := initEvent.Timestamp.Add(r.MaxDelay)
		
		for _, event := range eventList {
			if event.EventType == r.ResponseEvent && 
			   event.NodeID != initEvent.NodeID &&
			   event.Timestamp.After(initEvent.Timestamp) &&
			   event.Timestamp.Before(cutoffTime) {
				
				// Check if this response corresponds to the initiator event
				if r.isValidResponse(initEvent, event) {
					validResponses++
				}
			}
		}
		
		if uint16(validResponses) < r.MinResponders {
			errors = append(errors, ValidationError{
				Rule:        r.RuleDescription,
				RuleDescription: fmt.Sprintf("Initiator node %d got only %d responses, minimum %d required", 
					initEvent.NodeID, validResponses, r.MinResponders),
				Events:      []events.ConsensusEvent{initEvent},
				Severity:    SeverityError,
				NodeID:      initEvent.NodeID,
			})
		}
	}
	
	return errors
}

// isValidResponse checks if a response event corresponds to an initiator event
func (r *CrossNodeCoordinationRule) isValidResponse(initiator, response events.ConsensusEvent) bool {
	// Check if events are related (same view, block hash, etc.)
	initiatorView := getViewFromEvent(initiator)
	responseView := getViewFromEvent(response)
	return initiatorView == responseView
}

// Description returns rule description
func (r *CrossNodeCoordinationRule) Description() string {
	return r.RuleDescription
}

// StateTransitionRule validates proper state machine transitions
type StateTransitionRule struct {
	FromState   string                     `json:"from_state"`
	ToState     string                     `json:"to_state"`
	ValidEvents []events.EventType     `json:"valid_events"`
	RuleDescription string                     `json:"description"`
}

// Validate checks that state transitions are triggered by valid events
func (r *StateTransitionRule) Validate(eventList []events.ConsensusEvent) []ValidationError {
	errors := []ValidationError{}
	nodeStates := make(map[uint16]string)
	
	for _, event := range eventList {
		if event.EventType == events.EventStateTransition {
			fromState := event.FromState
			toState := event.ToState
			
			if string(fromState) == r.FromState && string(toState) == r.ToState {
				// Check if transition was triggered by valid event
				if !r.isValidTriggerEvent(event.Trigger) {
					errors = append(errors, ValidationError{
						Rule:        r.RuleDescription,
						RuleDescription: fmt.Sprintf("Node %d: Invalid state transition %s->%s triggered by %s", 
							event.NodeID, fromState, toState, event.Trigger),
						Events:      []events.ConsensusEvent{event},
						Severity:    SeverityError,
						NodeID:      event.NodeID,
					})
				}
			}
			
			nodeStates[event.NodeID] = string(toState)
		}
	}
	
	return errors
}

// isValidTriggerEvent checks if the trigger is in the list of valid events
func (r *StateTransitionRule) isValidTriggerEvent(trigger string) bool {
	for _, validEvent := range r.ValidEvents {
		if string(validEvent) == trigger {
			return true
		}
	}
	return false
}

// Description returns rule description
func (r *StateTransitionRule) Description() string {
	return r.RuleDescription
}

// Helper functions for rule validation

// groupEventsByNode organizes events by node ID for easier analysis
func groupEventsByNode(eventList []events.ConsensusEvent) map[uint16][]events.ConsensusEvent {
	nodeEvents := make(map[uint16][]events.ConsensusEvent)
	for _, event := range eventList {
		nodeEvents[event.NodeID] = append(nodeEvents[event.NodeID], event)
	}
	return nodeEvents
}

// getViewFromEvent extracts view number from event payload
func getViewFromEvent(event events.ConsensusEvent) uint64 {
	if view, ok := event.Payload["view"].(uint64); ok {
		return view
	}
	if view, ok := event.Payload["view"].(types.ViewNumber); ok {
		return uint64(view)
	}
	return 0
}

// calculateLeader determines which node should be leader for given view
func calculateLeader(view uint64, totalNodes uint16) uint16 {
	return uint16(view % uint64(totalNodes))
}

// getTotalNodes returns the total number of nodes in the consensus network
// This should be configurable in a real implementation
func getTotalNodes() uint16 {
	return 5 // Default for testing - should be made configurable
}

// ValidateEvents runs all validation rules against a set of events
func ValidateEvents(eventList []events.ConsensusEvent, rules []ValidationRule) []ValidationError {
	var allErrors []ValidationError
	
	for _, rule := range rules {
		errors := rule.Validate(eventList)
		allErrors = append(allErrors, errors...)
	}
	
	return allErrors
}