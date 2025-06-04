package main

import (
	"fmt"
	"log"

	"btc-federation/internal/config"
)

func main() {
	cfg, err := config.LoadConfig("conf.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("BTC Federation Node starting with config: %+v\n", cfg)
	
	// TODO: Initialize network manager, peer manager, etc.
	fmt.Println("Node started successfully")
}