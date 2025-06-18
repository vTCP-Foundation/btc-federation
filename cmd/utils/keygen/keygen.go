package main

import (
	"flag"
	"fmt"
	"os"

	"btc-federation/internal/keys"
)

func main() {
	var (
		privateKey = flag.String("private", "", "Get public key from private key (base64)")
		helpFlag   = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *helpFlag {
		showHelp()
		return
	}

	km := keys.NewKeyManager()

	if *privateKey != "" {
		getPublicKey(km, *privateKey)
		return
	}

	// Default: generate a new key pair
	generateKeyPair(km)
}

func showHelp() {
	fmt.Println("BTC Federation Key Generator")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  keygen                    Generate a new key pair")
	fmt.Println("  keygen -private <key>     Get public key from private key")
	fmt.Println("  keygen -help              Show this help")
}

func generateKeyPair(km *keys.KeyManager) {
	privateKey, err := km.GeneratePrivateKey()
	if err != nil {
		fmt.Printf("Error generating private key: %v\n", err)
		os.Exit(1)
	}

	publicKey, err := km.GetPublicKey(privateKey)
	if err != nil {
		fmt.Printf("Error deriving public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Private Key: %s\n", privateKey)
	fmt.Printf("Public Key:  %s\n", publicKey)
}

func getPublicKey(km *keys.KeyManager, privateKey string) {
	publicKey, err := km.GetPublicKey(privateKey)
	if err != nil {
		fmt.Printf("Error getting public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Public Key: %s\n", publicKey)
}
