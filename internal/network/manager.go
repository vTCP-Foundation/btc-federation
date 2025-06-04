package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	"btc-federation/internal/types"
)

// Manager implements the NetworkManager interface
type Manager struct {
	// Configuration
	config *types.NetworkConfig

	// libp2p host wrapper
	hostWrapper *HostWrapper

	// Protocol handlers
	protocolHandlers map[protocol.ID]ProtocolHandler
	protocolMutex    sync.RWMutex

	// Connection tracking
	connections      map[peer.ID]*ConnectionInfo
	connectionsMutex sync.RWMutex

	// Event handlers
	connectionHandlers      []ConnectionEventHandler
	connectionHandlersMutex sync.RWMutex

	// State management
	isStarted  bool
	startMutex sync.Mutex

	// Context for operations
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates a new network manager with the given configuration
func NewManager(config *types.NetworkConfig, privateKeyBase64 string) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Convert private key to libp2p format
	var privateKey crypto.PrivKey
	if privateKeyBase64 != "" {
		key, err := ConvertPrivateKeyFromBase64(privateKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert private key: %w", err)
		}
		privateKey = key
	}

	// Create host wrapper
	hostWrapper, err := NewHostWrapper(config, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create host wrapper: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		config:           config,
		hostWrapper:      hostWrapper,
		protocolHandlers: make(map[protocol.ID]ProtocolHandler),
		connections:      make(map[peer.ID]*ConnectionInfo),
		ctx:              ctx,
		cancel:           cancel,
	}

	return manager, nil
}

// Start starts the network manager and begins listening for connections
func (m *Manager) Start(ctx context.Context) error {
	m.startMutex.Lock()
	defer m.startMutex.Unlock()

	if m.isStarted {
		return fmt.Errorf("network manager is already started")
	}

	// Register network event handlers
	m.hostWrapper.Host().Network().Notify(&networkNotifiee{manager: m})

	// Register default protocol handler for peer exchange (Task 2.4 preparation)
	m.registerDefaultProtocols()

	m.isStarted = true

	// Log successful start
	// TODO: Use proper logging system
	fmt.Printf("Network manager started successfully. Listening on: %v\n", m.hostWrapper.Host().Addrs())

	return nil
}

// Stop stops the network manager and closes all connections
func (m *Manager) Stop() error {
	m.startMutex.Lock()
	defer m.startMutex.Unlock()

	if !m.isStarted {
		return nil // Already stopped
	}

	// Cancel context to stop all operations
	m.cancel()

	// Close host wrapper
	if err := m.hostWrapper.Close(); err != nil {
		return fmt.Errorf("failed to close host wrapper: %w", err)
	}

	// Clear connections
	m.connectionsMutex.Lock()
	m.connections = make(map[peer.ID]*ConnectionInfo)
	m.connectionsMutex.Unlock()

	m.isStarted = false

	return nil
}

// Connect establishes a connection to a peer at the given address
func (m *Manager) Connect(ctx context.Context, addr multiaddr.Multiaddr) error {
	if !m.isStarted {
		return fmt.Errorf("network manager is not started")
	}

	return m.hostWrapper.Connect(ctx, addr)
}

// Disconnect disconnects from the specified peer
func (m *Manager) Disconnect(peerID peer.ID) error {
	if !m.isStarted {
		return fmt.Errorf("network manager is not started")
	}

	return m.hostWrapper.Host().Network().ClosePeer(peerID)
}

// GetConnections returns information about all current connections
func (m *Manager) GetConnections() []ConnectionInfo {
	m.connectionsMutex.RLock()
	defer m.connectionsMutex.RUnlock()

	connections := make([]ConnectionInfo, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, *conn)
	}

	return connections
}

// GetHost returns the underlying libp2p host
func (m *Manager) GetHost() host.Host {
	return m.hostWrapper.Host()
}

// RegisterProtocolHandler registers a handler for a specific protocol
func (m *Manager) RegisterProtocolHandler(protocolID protocol.ID, handler ProtocolHandler) error {
	m.protocolMutex.Lock()
	defer m.protocolMutex.Unlock()

	if _, exists := m.protocolHandlers[protocolID]; exists {
		return fmt.Errorf("protocol handler for %s already registered", protocolID)
	}

	m.protocolHandlers[protocolID] = handler

	// Register with libp2p host
	m.hostWrapper.Host().SetStreamHandler(protocolID, func(stream network.Stream) {
		if err := handler.HandleStream(stream); err != nil {
			// TODO: Use proper logging
			fmt.Printf("Error handling stream for protocol %s: %v\n", protocolID, err)
			stream.Reset()
		}
	})

	return nil
}

// UnregisterProtocolHandler unregisters a protocol handler
func (m *Manager) UnregisterProtocolHandler(protocolID protocol.ID) error {
	m.protocolMutex.Lock()
	defer m.protocolMutex.Unlock()

	if _, exists := m.protocolHandlers[protocolID]; !exists {
		return fmt.Errorf("protocol handler for %s not found", protocolID)
	}

	delete(m.protocolHandlers, protocolID)

	// Remove from libp2p host
	m.hostWrapper.Host().RemoveStreamHandler(protocolID)

	return nil
}

// RegisterConnectionHandler registers a connection event handler
func (m *Manager) RegisterConnectionHandler(handler ConnectionEventHandler) error {
	m.connectionHandlersMutex.Lock()
	defer m.connectionHandlersMutex.Unlock()

	m.connectionHandlers = append(m.connectionHandlers, handler)
	return nil
}

// UnregisterConnectionHandler unregisters a connection event handler
func (m *Manager) UnregisterConnectionHandler(handler ConnectionEventHandler) error {
	m.connectionHandlersMutex.Lock()
	defer m.connectionHandlersMutex.Unlock()

	for i, h := range m.connectionHandlers {
		if h == handler {
			m.connectionHandlers = append(m.connectionHandlers[:i], m.connectionHandlers[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("connection handler not found")
}

// UpdateConfig updates the network configuration (for Task 2.5 hot-reload)
func (m *Manager) UpdateConfig(config *types.NetworkConfig) error {
	// For Task 2.5 implementation - placeholder for now
	// This will be fully implemented in Task 2.5
	m.config = config
	return nil
}

// ValidateConfig validates a network configuration
func (m *Manager) ValidateConfig(config *types.NetworkConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if len(config.Addresses) == 0 {
		return fmt.Errorf("at least one listen address must be specified")
	}

	// Validate each address
	for i, addrStr := range config.Addresses {
		_, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return fmt.Errorf("invalid address at index %d: %w", i, err)
		}
	}

	return nil
}

// GetCurrentConfig returns the current network configuration
func (m *Manager) GetCurrentConfig() *types.NetworkConfig {
	return m.config
}

// registerDefaultProtocols registers the default protocols for peer exchange
func (m *Manager) registerDefaultProtocols() {
	// Register placeholder for vtcp/btc-foundation/v1.0.0 protocol
	// This will be fully implemented in Task 2.4
	protocolID := protocol.ID("/vtcp/btc-foundation/v1.0.0")
	m.hostWrapper.Host().SetStreamHandler(protocolID, func(stream network.Stream) {
		// Placeholder handler - will be replaced in Task 2.4
		defer stream.Close()
		// TODO: Implement proper peer exchange protocol
	})
}

// emitConnectionEvent emits a connection event to all registered handlers
func (m *Manager) emitConnectionEvent(eventType ConnectionEventType, peerID peer.ID, addr multiaddr.Multiaddr, err error) {
	m.connectionHandlersMutex.RLock()
	handlers := make([]ConnectionEventHandler, len(m.connectionHandlers))
	copy(handlers, m.connectionHandlers)
	m.connectionHandlersMutex.RUnlock()

	// Emit events asynchronously to avoid blocking
	go func() {
		for _, handler := range handlers {
			switch eventType {
			case EventConnected:
				if handlerErr := handler.OnPeerConnected(peerID, addr); handlerErr != nil {
					// TODO: Use proper logging
					fmt.Printf("Error in connection handler: %v\n", handlerErr)
				}
			case EventDisconnected, EventConnectionFailed:
				if handlerErr := handler.OnPeerDisconnected(peerID, err); handlerErr != nil {
					// TODO: Use proper logging
					fmt.Printf("Error in disconnection handler: %v\n", handlerErr)
				}
			}
		}
	}()
}

// updateConnectionState updates the connection state for a peer
func (m *Manager) updateConnectionState(peerID peer.ID, state ConnectionState, addr multiaddr.Multiaddr) {
	m.connectionsMutex.Lock()
	defer m.connectionsMutex.Unlock()

	now := time.Now()

	if conn, exists := m.connections[peerID]; exists {
		conn.State = state
		conn.LastSeen = now
		if addr != nil {
			conn.Address = addr
		}
	} else {
		m.connections[peerID] = &ConnectionInfo{
			PeerID:      peerID,
			Address:     addr,
			State:       state,
			ConnectedAt: now,
			LastSeen:    now,
		}
	}
}
