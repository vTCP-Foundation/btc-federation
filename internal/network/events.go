package network

import (
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"

	"btc-federation/internal/logger"
)

// networkNotifiee implements the libp2p network.Notifiee interface
// to handle connection events and update the manager's state
type networkNotifiee struct {
	manager *Manager
}

// Connected implements network.Notifiee
func (nn *networkNotifiee) Connected(n network.Network, conn network.Conn) {
	// This is called when a new connection is established
	peerID := conn.RemotePeer()
	remoteAddr := conn.RemoteMultiaddr()
	direction := conn.Stat().Direction

	logger.Info("NetworkManager: Connection established with peer",
		"peer_id", peerID,
		"address", remoteAddr,
		"direction", direction)

	connectionTime := conn.Stat().Opened
	now := time.Now()
	delay := now.Sub(connectionTime)
	logger.Debug("CONNECT DEBUG: Connection timing",
		"opened_at", connectionTime,
		"processing_at", now,
		"delay", delay)

	// Log all connections to this peer for debugging
	allConnsToPeer := n.ConnsToPeer(peerID)
	logger.Debug("LIBP2P DEBUG: Total connections to peer", "peer_id", peerID, "count", len(allConnsToPeer))
	for i, c := range allConnsToPeer {
		logger.Debug("LIBP2P DEBUG: Connection details",
			"index", i,
			"opened", c.Stat().Opened,
			"direction", c.Stat().Direction)
	}

	// Handle the connection
	nn.manager.emitConnectionEvent(EventConnected, peerID, remoteAddr, nil)

	// Special handling for incoming connections - try to adopt them if we have a matching peer handler
	if direction == network.DirInbound {
		nn.manager.handleIncomingConnection(peerID, conn)
	}

	// Handle connection deduplication if we have multiple connections to the same peer
	if len(allConnsToPeer) > 1 {
		// Check if we should deduplicate
		if nn.manager.handleConnectionDeduplication(conn) {
			return // Connection was closed due to deduplication
		}
	}

	logger.Info("NetworkManager: Successfully established connection with peer", "peer_id", peerID.String())
}

// Disconnected implements network.Notifiee
func (nn *networkNotifiee) Disconnected(n network.Network, conn network.Conn) {
	// This is called when a connection is closed
	peerID := conn.RemotePeer()
	remoteAddr := conn.RemoteMultiaddr()

	logger.Info("NetworkManager: Connection closed with peer",
		"peer_id", peerID,
		"address", remoteAddr)

	// Log connection details for debugging
	connStat := conn.Stat()
	logger.Debug("DISCONNECT DEBUG: Connection details",
		"direction", connStat.Direction,
		"opened", connStat.Opened,
		"duration", time.Since(connStat.Opened))

	// Check remaining connections
	remainingConns := n.ConnsToPeer(peerID)
	logger.Debug("DISCONNECT DEBUG: Remaining connections to peer",
		"peer_id", peerID,
		"count", len(remainingConns))

	// Emit disconnect event
	nn.manager.emitConnectionEvent(EventDisconnected, peerID, remoteAddr, nil)

	// Update connection state
	nn.manager.updateConnectionState(peerID, StateDisconnecting, remoteAddr)
}

// Listen implements network.Notifiee
func (nn *networkNotifiee) Listen(n network.Network, addr multiaddr.Multiaddr) {
	// This is called when we start listening on an address
	// Optionally log this event
	// logger.Info("Started listening on", "address", addr.String())
}

// ListenClose implements network.Notifiee
func (nn *networkNotifiee) ListenClose(n network.Network, addr multiaddr.Multiaddr) {
	// This is called when we stop listening on an address
	// Optionally log this event
	// logger.Info("Stopped listening on", "address", addr.String())
}
