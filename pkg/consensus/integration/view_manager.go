package integration

import (
	"fmt"
	"sync"
	"time"

	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/types"
)

const (
	// DefaultViewTimeout is the default timeout duration for a view
	DefaultViewTimeout = 30 * time.Second
	
	// DefaultTimeoutBackoff is the multiplier for timeout backoff
	DefaultTimeoutBackoff = 1.5
)

// ViewManager handles view advancement, timeouts, and leader rotation.
// It manages the view-based leader rotation mechanism of HotStuff.
type ViewManager struct {
	currentView   types.ViewNumber
	viewTimeout   time.Duration
	timer         *time.Timer
	leaderElection *LeaderElection
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Timeout configuration
	baseTimeout    time.Duration
	timeoutBackoff float64
	
	// Timeout callback
	onTimeout func(view types.ViewNumber)
	
	// View change callback
	onViewChange func(oldView, newView types.ViewNumber)
	
	// Timeout certificates for view justification
	timeoutCerts map[types.ViewNumber][]messages.TimeoutMsg
	
	// Stop channel for graceful shutdown
	stopCh chan struct{}
}

// NewViewManager creates a new view manager.
func NewViewManager(leaderElection *LeaderElection) *ViewManager {
	return &ViewManager{
		currentView:    0,
		viewTimeout:    DefaultViewTimeout,
		leaderElection: leaderElection,
		baseTimeout:    DefaultViewTimeout,
		timeoutBackoff: DefaultTimeoutBackoff,
		timeoutCerts:   make(map[types.ViewNumber][]messages.TimeoutMsg),
		stopCh:         make(chan struct{}),
	}
}

// Start initializes the view manager and starts the timeout timer.
func (vm *ViewManager) Start() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	vm.resetTimeout()
	return nil
}

// Stop gracefully shuts down the view manager.
func (vm *ViewManager) Stop() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	if vm.timer != nil {
		vm.timer.Stop()
	}
	
	close(vm.stopCh)
	return nil
}

// GetCurrentView returns the current view number.
func (vm *ViewManager) GetCurrentView() types.ViewNumber {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.currentView
}

// GetCurrentLeader returns the leader for the current view.
func (vm *ViewManager) GetCurrentLeader() types.NodeID {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.leaderElection.GetLeader(vm.currentView)
}

// GetLeaderForView returns the leader for a specific view.
func (vm *ViewManager) GetLeaderForView(view types.ViewNumber) types.NodeID {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.leaderElection.GetLeader(view)
}

// IsLeader checks if the given node is the leader for the current view.
func (vm *ViewManager) IsLeader(nodeID types.NodeID) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	currentLeader := vm.leaderElection.GetLeader(vm.currentView)
	return nodeID == currentLeader
}

// AdvanceView advances to the next view.
func (vm *ViewManager) AdvanceView() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	oldView := vm.currentView
	vm.currentView++
	
	// Calculate new timeout with backoff
	vm.viewTimeout = time.Duration(float64(vm.baseTimeout) * vm.timeoutBackoff)
	
	// Reset the timeout timer for the new view
	vm.resetTimeout()
	
	// Call view change callback if set
	if vm.onViewChange != nil {
		vm.onViewChange(oldView, vm.currentView)
	}
}

// SyncToView synchronizes to a specific view number (used for catching up).
func (vm *ViewManager) SyncToView(targetView types.ViewNumber) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	if targetView <= vm.currentView {
		return fmt.Errorf("target view %d is not greater than current view %d", targetView, vm.currentView)
	}
	
	oldView := vm.currentView
	vm.currentView = targetView
	
	// Reset timeout for new view
	vm.resetTimeout()
	
	// Call view change callback if set
	if vm.onViewChange != nil {
		vm.onViewChange(oldView, vm.currentView)
	}
	
	return nil
}

// ProcessTimeout processes a timeout message from a node.
func (vm *ViewManager) ProcessTimeout(timeoutMsg *messages.TimeoutMsg) error {
	if timeoutMsg == nil {
		return fmt.Errorf("timeout message cannot be nil")
	}
	
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	// Store timeout certificate
	viewTimeouts := vm.timeoutCerts[timeoutMsg.ViewNumber]
	viewTimeouts = append(viewTimeouts, *timeoutMsg)
	vm.timeoutCerts[timeoutMsg.ViewNumber] = viewTimeouts
	
	// Check if we have enough timeout certificates to advance the view
	// In a real implementation, this would check for quorum threshold
	// For now, we'll advance if we get any timeout for current view
	if timeoutMsg.ViewNumber == vm.currentView {
		vm.handleTimeout(vm.currentView)
	}
	
	return nil
}

// ProcessNewView processes a new view announcement message.
func (vm *ViewManager) ProcessNewView(newViewMsg *messages.NewViewMsg) error {
	if newViewMsg == nil {
		return fmt.Errorf("new view message cannot be nil")
	}
	
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	// Validate the new view is advancing
	if newViewMsg.ViewNumber <= vm.currentView {
		return fmt.Errorf("new view %d is not greater than current view %d", 
			newViewMsg.ViewNumber, vm.currentView)
	}
	
	// Verify timeout certificates justify the view change
	if err := vm.validateTimeoutCertificates(newViewMsg); err != nil {
		return fmt.Errorf("invalid timeout certificates: %w", err)
	}
	
	// Advance to the new view
	oldView := vm.currentView
	vm.currentView = newViewMsg.ViewNumber
	
	// Reset timeout for new view
	vm.resetTimeout()
	
	// Call view change callback if set
	if vm.onViewChange != nil {
		vm.onViewChange(oldView, vm.currentView)
	}
	
	return nil
}

// SetTimeoutCallback sets a callback function that will be called on view timeouts.
func (vm *ViewManager) SetTimeoutCallback(callback func(view types.ViewNumber)) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.onTimeout = callback
}

// SetViewChangeCallback sets a callback function that will be called on view changes.
func (vm *ViewManager) SetViewChangeCallback(callback func(oldView, newView types.ViewNumber)) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.onViewChange = callback
}

// ResetTimeout resets the view timeout timer.
func (vm *ViewManager) ResetTimeout() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.resetTimeout()
}

// GetTimeoutCertificates returns timeout certificates for a specific view.
func (vm *ViewManager) GetTimeoutCertificates(view types.ViewNumber) []messages.TimeoutMsg {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	if certs, exists := vm.timeoutCerts[view]; exists {
		// Return a copy to prevent external modification
		result := make([]messages.TimeoutMsg, len(certs))
		copy(result, certs)
		return result
	}
	
	return nil
}

// resetTimeout resets the timeout timer (must be called with mutex held).
func (vm *ViewManager) resetTimeout() {
	if vm.timer != nil {
		vm.timer.Stop()
	}
	
	vm.timer = time.AfterFunc(vm.viewTimeout, func() {
		vm.handleTimeout(vm.currentView)
	})
}

// handleTimeout handles a view timeout event.
func (vm *ViewManager) handleTimeout(view types.ViewNumber) {
	vm.mu.Lock()
	currentView := vm.currentView
	vm.mu.Unlock()
	
	// Only handle timeout if it's for the current view
	if view != currentView {
		return
	}
	
	// Call timeout callback if set
	if vm.onTimeout != nil {
		vm.onTimeout(view)
	}
	
	// Advance to next view
	vm.AdvanceView()
}

// validateTimeoutCertificates validates that timeout certificates justify a view change.
func (vm *ViewManager) validateTimeoutCertificates(newViewMsg *messages.NewViewMsg) error {
	// In a real implementation, this would verify:
	// 1. Sufficient number of timeout certificates (quorum threshold)
	// 2. All certificates are for the previous view
	// 3. All signatures are valid
	
	expectedPrevView := newViewMsg.ViewNumber - 1
	
	for i, timeout := range newViewMsg.TimeoutCerts {
		if timeout.ViewNumber != expectedPrevView {
			return fmt.Errorf("timeout certificate %d has view %d, expected %d", 
				i, timeout.ViewNumber, expectedPrevView)
		}
	}
	
	return nil
}

// String returns a string representation of the view manager state.
func (vm *ViewManager) String() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	leader := vm.leaderElection.GetLeader(vm.currentView)
	
	return fmt.Sprintf("ViewManager{View: %d, Leader: %d, Timeout: %v}", 
		vm.currentView, leader, vm.viewTimeout)
}