package main

import (
	"flag"
	"fmt"
	"os"

	"btc-federation/internal/keys"
	"btc-federation/internal/logger"
)

func main() {
	// Initialize logger with default console output for utility
	loggerConfig := logger.Config{
		ConsoleOutput: true,
		ConsoleColor:  false,
		FileOutput:    false,
		Level:         "error", // Only log errors for utilities
	}
	logger.Init(loggerConfig)

	helpFlag := flag.Bool("help", false, "Show help message")
	privateFlag := flag.String("private", "", "Private key to get public key from")
	flag.Parse()

	if *helpFlag {
		fmt.Println("BTC Federation Key Generator")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  keygen                    Generate a new Ed25519 key pair")
		fmt.Println("  keygen -private <key>     Get public key from private key")
		fmt.Println("  keygen -help              Show this help")
		return
	}

	// Initialize key manager
	keyManager := keys.NewKeyManager()

	if *privateFlag != "" {
		// Get public key from private key
		publicKey, err := keyManager.GetPublicKey(*privateFlag)
		if err != nil {
			logger.Error("Error getting public key", "error", err)
			os.Exit(1)
		}

		fmt.Printf("Public Key: %s\n", publicKey)
		return
	}

	// Generate new Ed25519 key pair
	privateKey, err := keyManager.GeneratePrivateKey()
	if err != nil {
		logger.Error("Error generating private key", "error", err)
		os.Exit(1)
	}

	publicKey, err := keyManager.GetPublicKey(privateKey)
	if err != nil {
		logger.Error("Error getting public key", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Private Key: %s\n", privateKey)
	fmt.Printf("Public Key:  %s\n", publicKey)
}
