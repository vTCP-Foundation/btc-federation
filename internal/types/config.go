package types

import "time"

// Config represents the complete application configuration
type Config struct {
	Node    NodeConfig    `yaml:"node" validate:"required"`
	Network NetworkConfig `yaml:"network" validate:"required"`
	Peers   PeersConfig   `yaml:"peers" validate:"required"`
	Logging LoggingConfig `yaml:"logging" validate:"required"`
}

// NodeConfig contains node-specific configuration
type NodeConfig struct {
	PrivateKey string `yaml:"private_key" validate:"omitempty,base64"`
}

// NetworkConfig contains network-related configuration
type NetworkConfig struct {
	Addresses []string `yaml:"addresses" validate:"required,min=1,dive,required"`
}

// PeersConfig contains peer management configuration
type PeersConfig struct {
	ConnectionTimeout time.Duration `yaml:"connection_timeout" validate:"required,min=1s"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" validate:"required,oneof=debug info warn error"`
	Format string `yaml:"format" validate:"required,oneof=json text"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			PrivateKey: "", // Will be generated if empty
		},
		Network: NetworkConfig{
			Addresses: []string{
				"/ip4/0.0.0.0/tcp/9000",
				"/ip6/::/tcp/9000",
			},
		},
		Peers: PeersConfig{
			ConnectionTimeout: 10 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}
