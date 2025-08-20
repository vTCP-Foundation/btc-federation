// Package integration provides component integration framework for the HotStuff consensus protocol.
package integration

import (
	"context"
	"fmt"
	"sync"

	"btc-federation/pkg/consensus/crypto"
	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/network"
	"btc-federation/pkg/consensus/storage"
	"btc-federation/pkg/consensus/types"
)

// MessageHandler routes and processes different consensus message types.
// It serves as the central message processing hub for the consensus system.
type MessageHandler struct {
	consensus    *engine.HotStuffConsensus
	phaseManager *PhaseManager
	viewManager  *ViewManager
	network      network.NetworkInterface
	storage      storage.StorageInterface
	crypto       crypto.CryptoInterface
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Channel for stopping the message handler
	stopCh chan struct{}
	
	// Wait group for graceful shutdown
	wg sync.WaitGroup
}

// NewMessageHandler creates a new message handler with all required dependencies.
func NewMessageHandler(
	consensus *engine.HotStuffConsensus,
	phaseManager *PhaseManager,
	viewManager *ViewManager,
	network network.NetworkInterface,
	storage storage.StorageInterface,
	crypto crypto.CryptoInterface,
) *MessageHandler {
	return &MessageHandler{
		consensus:    consensus,
		phaseManager: phaseManager,
		viewManager:  viewManager,
		network:      network,
		storage:      storage,
		crypto:       crypto,
		stopCh:       make(chan struct{}),
	}
}

// Start begins message processing in a separate goroutine.
func (mh *MessageHandler) Start() error {
	if mh.network == nil {
		return fmt.Errorf("network interface cannot be nil")
	}
	
	mh.wg.Add(1)
	go mh.messageLoop()
	
	return nil
}

// Stop gracefully shuts down the message handler.
func (mh *MessageHandler) Stop() error {
	close(mh.stopCh)
	mh.wg.Wait()
	return nil
}

// messageLoop is the main message processing loop.
func (mh *MessageHandler) messageLoop() {
	defer mh.wg.Done()
	
	receiveCh := mh.network.Receive()
	
	for {
		select {
		case <-mh.stopCh:
			return
			
		case received := <-receiveCh:
			if received.Message == nil {
				continue
			}
			
			if err := mh.processMessage(received.Message); err != nil {
				// In a real implementation, this would use proper logging
				fmt.Printf("Error processing message from node %d: %v\n", received.Sender, err)
			}
		}
	}
}

// processMessage dispatches messages to appropriate handlers based on type.
func (mh *MessageHandler) processMessage(msg messages.ConsensusMessage) error {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	
	// Validate message view against current view
	currentView := mh.viewManager.GetCurrentView()
	messageView := msg.View()
	
	// Allow messages from current view or future views (for view synchronization)
	if messageView < currentView {
		return fmt.Errorf("received message from old view %d, current view is %d", messageView, currentView)
	}
	
	switch msg.Type() {
	case messages.MsgTypeProposal:
		return mh.HandleProposal(msg.(*messages.ProposalMsg))
	case messages.MsgTypeVote:
		return mh.HandleVote(msg.(*messages.VoteMsg))
	case messages.MsgTypeTimeout:
		return mh.HandleTimeout(msg.(*messages.TimeoutMsg))
	case messages.MsgTypeNewView:
		return mh.HandleNewView(msg.(*messages.NewViewMsg))
	default:
		return fmt.Errorf("unknown message type: %v", msg.Type())
	}
}

// HandleProposal processes block proposal messages.
func (mh *MessageHandler) HandleProposal(msg *messages.ProposalMsg) error {
	if msg == nil {
		return fmt.Errorf("proposal message cannot be nil")
	}
	
	// Verify we're in the correct view
	currentView := mh.viewManager.GetCurrentView()
	if msg.View() != currentView {
		// If message is from future view, trigger view synchronization
		if msg.View() > currentView {
			return mh.viewManager.SyncToView(msg.View())
		}
		return fmt.Errorf("proposal view %d does not match current view %d", msg.View(), currentView)
	}
	
	// Verify sender is the expected leader
	if !mh.viewManager.IsLeader(msg.Sender()) {
		return fmt.Errorf("proposal from non-leader node %d", msg.Sender())
	}
	
	// Process proposal through phase manager
	vote, err := mh.phaseManager.ProcessPrepare(msg)
	if err != nil {
		return fmt.Errorf("failed to process prepare: %w", err)
	}
	
	// If we got a vote, we would broadcast it in production
	// For demo purposes, we handle it internally
	_ = vote
	
	return nil
}

// HandleVote processes vote messages.
func (mh *MessageHandler) HandleVote(msg *messages.VoteMsg) error {
	if msg == nil {
		return fmt.Errorf("vote message cannot be nil")
	}
	
	// Verify vote signature
	if err := mh.verifyVoteSignature(msg.Vote); err != nil {
		return fmt.Errorf("invalid vote signature: %w", err)
	}
	
	// Process vote through consensus engine
	qc, err := mh.consensus.ProcessVote(msg.Vote)
	if err != nil {
		return fmt.Errorf("failed to process vote: %w", err)
	}
	
	// If a quorum certificate was formed, process it through phase manager
	if qc != nil {
		return mh.phaseManager.ProcessQuorumCertificate(qc)
	}
	
	return nil
}

// HandleTimeout processes view timeout messages.
func (mh *MessageHandler) HandleTimeout(msg *messages.TimeoutMsg) error {
	if msg == nil {
		return fmt.Errorf("timeout message cannot be nil")
	}
	
	// Verify timeout signature
	if err := mh.verifyTimeoutSignature(msg); err != nil {
		return fmt.Errorf("invalid timeout signature: %w", err)
	}
	
	// Process timeout through view manager
	return mh.viewManager.ProcessTimeout(msg)
}

// HandleNewView processes new view announcement messages.
func (mh *MessageHandler) HandleNewView(msg *messages.NewViewMsg) error {
	if msg == nil {
		return fmt.Errorf("new view message cannot be nil")
	}
	
	// Verify sender is the expected leader for the new view
	expectedLeader := mh.viewManager.GetLeaderForView(msg.View())
	if msg.Sender() != expectedLeader {
		return fmt.Errorf("new view message from non-leader node %d, expected leader %d", msg.Sender(), expectedLeader)
	}
	
	// Process new view through view manager
	return mh.viewManager.ProcessNewView(msg)
}

// SendMessage broadcasts a consensus message to all nodes.
func (mh *MessageHandler) SendMessage(ctx context.Context, msg messages.ConsensusMessage) error {
	return mh.network.Broadcast(ctx, msg)
}

// SendMessageToNode sends a consensus message to a specific node.
func (mh *MessageHandler) SendMessageToNode(ctx context.Context, nodeID types.NodeID, msg messages.ConsensusMessage) error {
	return mh.network.Send(ctx, nodeID, msg)
}

// verifyVoteSignature verifies the cryptographic signature of a vote.
func (mh *MessageHandler) verifyVoteSignature(vote *types.Vote) error {
	// Create vote data for signature verification
	voteData := fmt.Sprintf("%x:%d:%s:%d", vote.BlockHash, vote.View, vote.Phase, vote.Voter)
	return mh.crypto.Verify([]byte(voteData), vote.Signature, vote.Voter)
}

// verifyTimeoutSignature verifies the cryptographic signature of a timeout message.
func (mh *MessageHandler) verifyTimeoutSignature(timeout *messages.TimeoutMsg) error {
	// Create timeout data for signature verification
	timeoutData := fmt.Sprintf("%d:%d", timeout.View(), timeout.Sender())
	return mh.crypto.Verify([]byte(timeoutData), timeout.Signature, timeout.Sender())
}

// GetConsensus returns the consensus engine for testing purposes.
func (mh *MessageHandler) GetConsensus() *engine.HotStuffConsensus {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.consensus
}

// GetPhaseManager returns the phase manager for testing purposes.
func (mh *MessageHandler) GetPhaseManager() *PhaseManager {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.phaseManager
}

// GetViewManager returns the view manager for testing purposes.
func (mh *MessageHandler) GetViewManager() *ViewManager {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.viewManager
}