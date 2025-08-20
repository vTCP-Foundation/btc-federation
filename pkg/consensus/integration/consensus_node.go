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

// NodeConfig contains configuration for creating a consensus node.
type NodeConfig struct {
	NodeID     types.NodeID
	Validators []types.NodeID
	Config     *types.ConsensusConfig
}

// ConsensusNode serves as the main dependency injection container for the consensus system.
// It wires together all components and provides the main entry points for consensus operations.
type ConsensusNode struct {
	// Node configuration
	nodeID types.NodeID
	config *types.ConsensusConfig
	
	// Core consensus components
	consensus      *engine.HotStuffConsensus
	messageHandler *MessageHandler
	phaseManager   *PhaseManager
	viewManager    *ViewManager
	leaderElection *LeaderElection
	
	// Infrastructure interfaces
	network network.NetworkInterface
	storage storage.StorageInterface
	crypto  crypto.CryptoInterface
	
	// State management
	mu      sync.RWMutex
	started bool
	
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewConsensusNode creates a new consensus node with dependency injection.
func NewConsensusNode(config NodeConfig, network network.NetworkInterface, storage storage.StorageInterface, crypto crypto.CryptoInterface) (*ConsensusNode, error) {
	if config.Config == nil {
		return nil, fmt.Errorf("consensus configuration cannot be nil")
	}
	
	if network == nil {
		return nil, fmt.Errorf("network interface cannot be nil")
	}
	
	if storage == nil {
		return nil, fmt.Errorf("storage interface cannot be nil")
	}
	
	if crypto == nil {
		return nil, fmt.Errorf("crypto interface cannot be nil")
	}
	
	if len(config.Validators) == 0 {
		return nil, fmt.Errorf("validator list cannot be empty")
	}
	
	if !config.Config.IsValidNodeID(config.NodeID) {
		return nil, fmt.Errorf("invalid node ID: %d", config.NodeID)
	}
	
	// Create consensus engine
	consensus, err := engine.NewHotStuffConsensus(config.NodeID, config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consensus engine: %w", err)
	}
	
	// Create leader election
	leaderElection, err := NewLeaderElection(config.Validators)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader election: %w", err)
	}
	
	// Create view manager
	viewManager := NewViewManager(leaderElection)
	
	// Create phase manager with infrastructure
	phaseManager := NewPhaseManager(consensus, viewManager, network, crypto)
	
	// Create message handler
	messageHandler := NewMessageHandler(consensus, phaseManager, viewManager, network, storage, crypto)
	
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	
	node := &ConsensusNode{
		nodeID:         config.NodeID,
		config:         config.Config,
		consensus:      consensus,
		messageHandler: messageHandler,
		phaseManager:   phaseManager,
		viewManager:    viewManager,
		leaderElection: leaderElection,
		network:        network,
		storage:        storage,
		crypto:         crypto,
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// Set up component callbacks
	node.setupCallbacks()
	
	return node, nil
}

// Start initializes and starts all consensus components.
func (cn *ConsensusNode) Start() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	
	if cn.started {
		return fmt.Errorf("consensus node already started")
	}
	
	// Start components in dependency order
	if err := cn.viewManager.Start(); err != nil {
		return fmt.Errorf("failed to start view manager: %w", err)
	}
	
	if err := cn.messageHandler.Start(); err != nil {
		return fmt.Errorf("failed to start message handler: %w", err)
	}
	
	cn.started = true
	return nil
}

// Stop gracefully shuts down all consensus components.
func (cn *ConsensusNode) Stop() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	
	if !cn.started {
		return nil // Already stopped
	}
	
	// Cancel context to signal shutdown
	cn.cancel()
	
	// Stop components in reverse dependency order
	var lastError error
	
	if err := cn.messageHandler.Stop(); err != nil {
		lastError = fmt.Errorf("failed to stop message handler: %w", err)
	}
	
	if err := cn.viewManager.Stop(); err != nil {
		if lastError != nil {
			lastError = fmt.Errorf("%v; failed to stop view manager: %w", lastError, err)
		} else {
			lastError = fmt.Errorf("failed to stop view manager: %w", err)
		}
	}
	
	cn.started = false
	return lastError
}

// ProcessMessage is the main entry point for processing consensus messages.
func (cn *ConsensusNode) ProcessMessage(msg messages.ConsensusMessage) error {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	
	if !cn.started {
		return fmt.Errorf("consensus node is not started")
	}
	
	return cn.messageHandler.processMessage(msg)
}

// ProposeBlock proposes a new block (only valid if this node is the current leader).
func (cn *ConsensusNode) ProposeBlock(payload []byte) error {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	
	if !cn.started {
		return fmt.Errorf("consensus node is not started")
	}
	
	// Check if we are the current leader
	if !cn.viewManager.IsLeader(cn.nodeID) {
		return fmt.Errorf("node %d is not the leader for current view %d", cn.nodeID, cn.viewManager.GetCurrentView())
	}
	
	// Follow HotStuff data flow diagram lines 20-29: Block Proposal Phase
	
	// Line 21-22: Get highest QC from block tree
	blockTree := cn.consensus.GetBlockTree()
	highestQC := cn.consensus.GetPrepareQC() // Get highest known QC
	
	// Line 23-24: Get parent block from storage
	parentBlock := blockTree.GetCommitted() // Use highest committed block as parent
	if parentBlock == nil {
		return fmt.Errorf("no committed parent block available")
	}
	
	// Line 25: Create new block with QC
	currentView := cn.viewManager.GetCurrentView()
	parentHash := parentBlock.Hash
	height := parentBlock.Height + 1
	
	block := types.NewBlock(parentHash, height, currentView, cn.nodeID, payload)
	
	// Line 26-27: Sign block proposal (LeaderSig)
	blockData := fmt.Sprintf("%x:%d:%d:%d", block.Hash, block.Height, block.View, block.Proposer)
	leaderSig, err := cn.crypto.Sign([]byte(blockData))
	if err != nil {
		return fmt.Errorf("failed to sign block proposal: %w", err)
	}
	_ = leaderSig // Used in production for proposal signature
	
	// Line 28-29: Broadcast ProposalMsg to all validators
	proposal := messages.NewProposalMsg(block, highestQC, currentView, cn.nodeID)
	// Add leader signature to proposal (should be in ProposalMsg structure)
	
	if err := cn.messageHandler.SendMessage(cn.ctx, proposal); err != nil {
		return fmt.Errorf("failed to broadcast proposal: %w", err)
	}
	
	// Leader must also process its own proposal through the same validation path
	// This ensures all nodes (including leader) follow the same protocol
	if err := cn.ProcessMessage(proposal); err != nil {
		return fmt.Errorf("leader failed to process own proposal: %w", err)
	}
	
	return nil
}

// GetNodeID returns this node's identifier.
func (cn *ConsensusNode) GetNodeID() types.NodeID {
	return cn.nodeID
}

// GetCurrentView returns the current consensus view.
func (cn *ConsensusNode) GetCurrentView() types.ViewNumber {
	return cn.viewManager.GetCurrentView()
}

// GetCurrentLeader returns the current leader.
func (cn *ConsensusNode) GetCurrentLeader() types.NodeID {
	return cn.viewManager.GetCurrentLeader()
}

// IsLeader returns true if this node is the current leader.
func (cn *ConsensusNode) IsLeader() bool {
	return cn.viewManager.IsLeader(cn.nodeID)
}

// GetCurrentPhase returns the current consensus phase.
func (cn *ConsensusNode) GetCurrentPhase() types.ConsensusPhase {
	return cn.phaseManager.GetCurrentPhase()
}

// GetConsensusState returns a snapshot of the current consensus state.
func (cn *ConsensusNode) GetConsensusState() ConsensusState {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	
	return ConsensusState{
		NodeID:        cn.nodeID,
		CurrentView:   cn.viewManager.GetCurrentView(),
		CurrentPhase:  cn.phaseManager.GetCurrentPhase(),
		CurrentLeader: cn.viewManager.GetCurrentLeader(),
		IsLeader:      cn.viewManager.IsLeader(cn.nodeID),
		IsStarted:     cn.started,
	}
}

// SyncToView synchronizes this node to a specific view (useful for demos and recovery).
func (cn *ConsensusNode) SyncToView(targetView types.ViewNumber) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	
	if !cn.started {
		return fmt.Errorf("consensus node is not started")
	}
	
	currentView := cn.viewManager.GetCurrentView()
	if targetView <= currentView {
		return nil // Already at or beyond target view
	}
	
	// Sync view manager
	if err := cn.viewManager.SyncToView(targetView); err != nil {
		return fmt.Errorf("failed to sync view manager: %w", err)
	}
	
	return nil
}

// GetBlockchainState returns detailed blockchain state information.
func (cn *ConsensusNode) GetBlockchainState() BlockchainStateInfo {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	
	blockTree := cn.consensus.GetBlockTree()
	
	return BlockchainStateInfo{
		NodeID:           cn.nodeID,
		BlockCount:       blockTree.GetBlockCount(),
		CommittedBlock:   blockTree.GetCommitted(),
		RootBlock:        blockTree.GetRoot(),
		LockedQC:         cn.consensus.GetLockedQC(),
		PrepareQC:        cn.consensus.GetPrepareQC(),
		ForkCount:        blockTree.GetForkCount(),
	}
}

// setupCallbacks configures inter-component callbacks.
func (cn *ConsensusNode) setupCallbacks() {
	// Set up view timeout callback
	cn.viewManager.SetTimeoutCallback(func(view types.ViewNumber) {
		// Create and broadcast timeout message
		timeoutMsg := messages.NewTimeoutMsg(
			view,
			cn.consensus.GetPrepareQC(),
			cn.nodeID,
			[]byte("mock_timeout_signature"),
		)
		
		// Broadcast timeout message (fire and forget)
		go func() {
			if err := cn.messageHandler.SendMessage(cn.ctx, timeoutMsg); err != nil {
				// Log error in real implementation
				fmt.Printf("Failed to broadcast timeout message: %v\n", err)
			}
		}()
	})
	
	// Set up view change callback
	cn.viewManager.SetViewChangeCallback(func(oldView, newView types.ViewNumber) {
		// Advance the consensus engine to the new view (in a goroutine to avoid deadlock)
		go func() {
			for cn.consensus.GetCurrentView() < newView {
				cn.consensus.AdvanceView()
			}
		}()
		
		// Reset phase manager for new view
		cn.phaseManager.Reset()
		
		// Check if we are the new leader without holding locks
		newLeader := cn.viewManager.GetLeaderForView(newView)
		if newLeader == cn.nodeID {
			// Create new view message
			timeoutCerts := cn.viewManager.GetTimeoutCertificates(oldView)
			newViewMsg := messages.NewNewViewMsg(
				newView,
				cn.consensus.GetPrepareQC(),
				timeoutCerts,
				cn.nodeID,
			)
			
			// Broadcast new view message (fire and forget)
			go func() {
				if err := cn.messageHandler.SendMessage(cn.ctx, newViewMsg); err != nil {
					// Log error in real implementation
					fmt.Printf("Failed to broadcast new view message: %v\n", err)
				}
			}()
		}
	})
}

// ConsensusState represents a snapshot of the consensus node state.
type ConsensusState struct {
	NodeID        types.NodeID
	CurrentView   types.ViewNumber
	CurrentPhase  types.ConsensusPhase
	CurrentLeader types.NodeID
	IsLeader      bool
	IsStarted     bool
}

// BlockchainStateInfo represents detailed blockchain state for verification.
type BlockchainStateInfo struct {
	NodeID           types.NodeID
	BlockCount       int
	CommittedBlock   *types.Block
	RootBlock        *types.Block
	LockedQC         *types.QuorumCertificate
	PrepareQC        *types.QuorumCertificate
	ForkCount        int
}

// String returns a string representation of the consensus state.
func (cs ConsensusState) String() string {
	return fmt.Sprintf("ConsensusState{NodeID: %d, View: %d, Phase: %s, Leader: %d, IsLeader: %t, Started: %t}",
		cs.NodeID, cs.CurrentView, cs.CurrentPhase, cs.CurrentLeader, cs.IsLeader, cs.IsStarted)
}

// String returns a string representation of the blockchain state.
func (bsi BlockchainStateInfo) String() string {
	committedHash := "nil"
	if bsi.CommittedBlock != nil {
		committedHash = fmt.Sprintf("%x", bsi.CommittedBlock.Hash)
	}
	
	return fmt.Sprintf("BlockchainState{NodeID: %d, Blocks: %d, Committed: %s, Forks: %d}",
		bsi.NodeID, bsi.BlockCount, committedHash, bsi.ForkCount)
}

// String returns a string representation of the consensus node.
func (cn *ConsensusNode) String() string {
	state := cn.GetConsensusState()
	return fmt.Sprintf("ConsensusNode{%s}", state.String())
}