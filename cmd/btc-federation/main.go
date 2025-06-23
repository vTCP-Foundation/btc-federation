package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"btc-federation/internal/config"
	"btc-federation/internal/keys"
	"btc-federation/internal/logger"
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

	// Initialize logger with configuration
	loggerConfig := logger.Config{
		ConsoleOutput: cfg.Logging.ConsoleOutput,
		ConsoleColor:  cfg.Logging.ConsoleColor,
		FileOutput:    cfg.Logging.FileOutput,
		FileName:      cfg.Logging.FileName,
		FileMaxSize:   cfg.Logging.FileMaxSize,
		Level:         cfg.Logging.Level,
	}

	if err := logger.Init(loggerConfig); err != nil {
		// Use standard logging as fallback since logger init failed
		panic("Failed to initialize logger: " + err.Error())
	}

	logger.Info("BTC Federation Node starting with config", "config", cfg)

	// Initialize network manager
	networkManager, err := network.NewManager(cfg)
	if err != nil {
		logger.Fatal("Failed to create network manager", "error", err)
	}

	// Start network manager
	ctx := context.Background()
	if err := networkManager.Start(ctx); err != nil {
		logger.Fatal("Failed to start network manager", "error", err)
	}

	logger.Info("Node started successfully")
	logger.Info("Peer ID", "peer_id", networkManager.GetHost().ID())
	logger.Info("Listening addresses", "addresses", networkManager.GetHost().Addrs())

	// Print public key for peer configuration
	if pubKey := networkManager.GetHost().ID(); pubKey != "" {
		logger.Info("For peers.yaml, use peer ID", "peer_id", pubKey)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down...")

	// Stop network manager
	if err := networkManager.Stop(); err != nil {
		logger.Error("Error stopping network manager", "error", err)
	}

	logger.Info("Shutdown complete")
}
