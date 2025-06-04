package network

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

// networkNotifiee implements the libp2p network.Notifiee interface
// to handle connection events and update the manager's state
type networkNotifiee struct {
	manager *Manager
}

// Connected is called when a connection is established
func (n *networkNotifiee) Connected(network network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	addr := conn.RemoteMultiaddr()

	// Update connection state
	n.manager.updateConnectionState(peerID, StateConnected, addr)

	// Emit connection event to handlers
	n.manager.emitConnectionEvent(EventConnected, peerID, addr, nil)
}

// Disconnected is called when a connection is closed
func (n *networkNotifiee) Disconnected(network network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	addr := conn.RemoteMultiaddr()

	// Update connection state
	n.manager.updateConnectionState(peerID, StateDisconnecting, addr)

	// Emit disconnection event to handlers
	n.manager.emitConnectionEvent(EventDisconnected, peerID, addr, nil)

	// Remove from connections map after a brief delay to allow handlers to process
	go func() {
		n.manager.connectionsMutex.Lock()
		delete(n.manager.connections, peerID)
		n.manager.connectionsMutex.Unlock()
	}()
}

// Listen is called when the network starts listening on an address
func (n *networkNotifiee) Listen(network network.Network, addr multiaddr.Multiaddr) {
	// TODO: Log that we're listening on this address
	// fmt.Printf("Started listening on: %s\n", addr.String())
}

// ListenClose is called when the network stops listening on an address
func (n *networkNotifiee) ListenClose(network network.Network, addr multiaddr.Multiaddr) {
	// TODO: Log that we stopped listening on this address
	// fmt.Printf("Stopped listening on: %s\n", addr.String())
}
