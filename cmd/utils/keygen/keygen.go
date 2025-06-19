package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"

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
		fmt.Println("  keygen                    Generate a new key pair")
		fmt.Println("  keygen -private <key>     Get public key from private key")
		fmt.Println("  keygen -help              Show this help")
		return
	}

	if *privateFlag != "" {
		// Get public key from private key
		privateKey, err := decodePrivateKey(*privateFlag)
		if err != nil {
			logger.Error("Error getting public key", "error", err)
			os.Exit(1)
		}

		publicKey, err := encodePublicKey(privateKey.GetPublic())
		if err != nil {
			logger.Error("Error getting public key", "error", err)
			os.Exit(1)
		}

		fmt.Printf("Public Key: %s\n", publicKey)
		return
	}

	// Generate new key pair
	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		logger.Error("Error generating private key", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Private Key: %s\n", privateKey)
	fmt.Printf("Public Key:  %s\n", publicKey)
}

func generateKeyPair() (string, string, error) {
	privateKey, _, err := crypto.GenerateKeyPair(crypto.ECDSA, -1)
	if err != nil {
		return "", "", err
	}

	privateKeyBytes, err := crypto.MarshalPrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}

	publicKeyBytes, err := crypto.MarshalPublicKey(privateKey.GetPublic())
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(privateKeyBytes), base64.StdEncoding.EncodeToString(publicKeyBytes), nil
}

func decodePrivateKey(encodedKey string) (crypto.PrivKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(decoded)
}

func encodePublicKey(publicKey crypto.PubKey) (string, error) {
	encoded, err := crypto.MarshalPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encoded), nil
}
