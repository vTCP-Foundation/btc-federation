package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"btc-federation/internal/config"
	"btc-federation/internal/keys"
	"btc-federation/internal/network"
)

func main() {
	// Initialize key manager
	keyManager := keys.NewKeyManager()

	// Initialize config manager
	configManager := config.NewManager(keyManager)

	// Load configuration
	cfg, err := configManager.LoadConfig("conf.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("BTC Federation Node starting with config: %+v\n", cfg)

	// Initialize network manager
	networkManager, err := network.NewManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create network manager: %v", err)
	}

	// Start network manager
	ctx := context.Background()
	if err := networkManager.Start(ctx); err != nil {
		log.Fatalf("Failed to start network manager: %v", err)
	}

	fmt.Println("Node started successfully")
	fmt.Printf("Peer ID: %s\n", networkManager.GetHost().ID())
	fmt.Printf("Listening addresses: %v\n", networkManager.GetHost().Addrs())
	
	// Print public key for peer configuration
	if pubKey := networkManager.GetHost().ID(); pubKey != "" {
		fmt.Printf("For peers.yaml, use peer ID: %s\n", pubKey)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// Stop network manager
	if err := networkManager.Stop(); err != nil {
		log.Printf("Error stopping network manager: %v", err)
	}

	fmt.Println("Shutdown complete")
}
