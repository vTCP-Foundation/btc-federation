// Package events defines the event tracing abstraction interfaces for consensus testing and monitoring.
// These interfaces allow the consensus engine to operate independently of specific event collection implementations.
package events

import (
	"time"
)

// EventTracer provides production-safe event tracing for consensus protocols.
// Production implementations should use NoOpEventTracer to avoid any overhead.
// Testing implementations can collect events for comprehensive validation.
type EventTracer interface {
	// RecordEvent records a consensus event with associated payload
	RecordEvent(nodeID uint16, eventType EventType, payload EventPayload)
	
	// RecordTransition records a state transition between consensus phases
	RecordTransition(nodeID uint16, from, to State, trigger string)
	
	// RecordMessage records message sending/receiving for network analysis
	RecordMessage(nodeID uint16, direction MessageDirection, msgType string, payload EventPayload)
}

// EventType represents the type of consensus event that occurred
type EventType string

const (
	// Proposal Events - Block proposal lifecycle
	EventProposalCreated   EventType = "proposal_created"
	EventProposalReceived  EventType = "proposal_received" 
	EventProposalValidated EventType = "proposal_validated"
	EventProposalRejected  EventType = "proposal_rejected"
	
	// Vote Events - Vote lifecycle and collection
	EventVoteCreated   EventType = "vote_created"
	EventVoteSent      EventType = "vote_sent"
	EventVoteReceived  EventType = "vote_received"
	EventVoteValidated EventType = "vote_validated"
	
	// QC Events - Quorum certificate formation and validation
	EventQCFormed     EventType = "qc_formed"
	EventQCReceived   EventType = "qc_received"
	EventQCValidated  EventType = "qc_validated"
	
	// View Events - View changes and timeouts
	EventViewTimeout    EventType = "view_timeout"
	EventViewChange     EventType = "view_change"
	EventNewViewStarted EventType = "new_view_started"
	
	// Byzantine Events - Malicious behavior detection
	EventByzantineDetected EventType = "byzantine_detected"
	EventEquivocationFound EventType = "equivocation_found"
	EventEvidenceStored    EventType = "evidence_stored"
	
	// Block Events - Block processing and commitment
	EventBlockCommitted  EventType = "block_committed"
	EventBlockAdded      EventType = "block_added"
	EventBlockValidated  EventType = "block_validated"
	
	// State Events - Internal state transitions
	EventStateTransition EventType = "state_transition"
	EventPhaseTransition EventType = "phase_transition"
)

// EventPayload contains event-specific data as key-value pairs
type EventPayload map[string]interface{}

// State represents consensus node internal states
type State string

const (
	StateIdle        State = "idle"
	StateProposing   State = "proposing"
	StateVoting      State = "voting"
	StateWaitingQC   State = "waiting_qc"
	StateCommitting  State = "committing"
	StateRecovering  State = "recovering"
	StatePartitioned State = "partitioned"
)

// MessageDirection indicates whether a message is being sent or received
type MessageDirection string

const (
	MessageInbound  MessageDirection = "inbound"
	MessageOutbound MessageDirection = "outbound"
)

// ConsensusEvent represents a single recorded event in the consensus protocol
type ConsensusEvent struct {
	NodeID     uint16           `json:"node_id"`
	EventType  EventType        `json:"event_type"`
	Payload    EventPayload     `json:"payload"`
	Timestamp  time.Time        `json:"timestamp"`
	
	// State transition specific fields
	FromState  State            `json:"from_state,omitempty"`
	ToState    State            `json:"to_state,omitempty"`
	Trigger    string           `json:"trigger,omitempty"`
	
	// Message specific fields
	Direction  MessageDirection `json:"direction,omitempty"`
	MessageType string          `json:"message_type,omitempty"`
}