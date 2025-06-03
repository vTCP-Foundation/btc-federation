# Bitcoin Federation - Fastnet Local Node

A Bitcoin Core regtest setup for fast development and testing of Bitcoin applications.

## Overview

This project provides a complete Bitcoin Core regtest environment with:
- Fast block generation (~10 seconds)
- Automatic mining capabilities
- Pre-configured RPC access
- Development-friendly wallet operations
- Transaction testing utilities

## Quick Start

### Prerequisites

- Linux x86_64 system
- Make utility
- Basic Unix tools (curl, tar, etc.)

### Installation

The Bitcoin Core binaries are already included in `.btc/node/`. The setup includes:
- Bitcoin Core v29.0
- Pre-configured regtest environment
- Automatic data directory setup

### Starting the Node

```bash
# Start the Bitcoin node
make btc-start

# Start the automatic miner (generates blocks every 10 seconds)
make btc-miner-start

# Check status
make btc-status
```

### Stopping Services

```bash
# Stop the miner
make btc-miner-stop

# Stop the node
make btc-stop

# Reset all data and start fresh
make btc-reset
```

## Available Commands

| Command | Description |
|---------|-------------|
| `make btc-start` | Start Bitcoin node in regtest mode |
| `make btc-stop` | Stop Bitcoin node |
| `make btc-reset` | Reset blockchain and restart |
| `make btc-miner-start` | Start automatic block generation |
| `make btc-miner-stop` | Stop automatic block generation |
| `make btc-status` | Show node and miner status |
| `make btc-logs` | Show recent logs |
| `make help` | Show all available commands |

## Development Usage

### Basic Wallet Operations

```bash
# Create a wallet
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 createwallet "dev_wallet"

# Get a new address
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getnewaddress

# Check balance
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getbalance

# Generate test Bitcoin (100 blocks to mature coinbase)
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 generatetoaddress 100 "YOUR_ADDRESS"
```

### Transaction Testing

```bash
# Send Bitcoin
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 sendtoaddress "RECIPIENT_ADDRESS" 1.5

# Check transaction status
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 gettransaction "TXID"
```

For detailed examples, see [`docs/bitcoin-fastnet-usage.md`](docs/bitcoin-fastnet-usage.md).

## Configuration

### Network Settings
- **Network**: regtest (isolated test network)
- **RPC Port**: 18443
- **Block Time**: ~10 seconds
- **RPC Access**: localhost only

### Security
- **RPC User**: fastnet
- **RPC Password**: fastnet123
- **⚠️ Development Only**: These credentials are not secure and should never be used in production

### File Structure
```
.btc/
├── node/                 # Bitcoin Core installation
│   ├── bin/             # Bitcoin binaries
│   └── bitcoin-fastnet.conf  # Configuration file
├── data/                # Blockchain data
└── miner.log           # Miner logs
```

## Features

### ✅ Implemented
- [x] Bitcoin Core v29.0 installation
- [x] Regtest network configuration
- [x] Fast block generation (10 seconds)
- [x] Automatic mining with start/stop controls
- [x] RPC interface setup
- [x] Wallet creation and management
- [x] Transaction execution
- [x] Balance checking
- [x] Transaction status verification
- [x] Comprehensive logging
- [x] Make-based command interface

### 🔧 Development Features
- Isolated regtest environment
- Pre-funded mining wallet
- Fast transaction confirmation
- Easy reset capabilities
- Detailed logging and monitoring

## Troubleshooting

### Common Issues

1. **Port 18443 already in use**
   ```bash
   sudo lsof -i :18443
   # Kill any conflicting processes
   ```

2. **Node won't start**
   ```bash
   make btc-logs  # Check for errors
   make btc-reset # Start fresh
   ```

3. **No spendable Bitcoin**
   ```bash
   # Generate blocks to mature coinbase transactions
   .btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 generatetoaddress 100 "YOUR_ADDRESS"
   ```

### Getting Help

```bash
# List all RPC commands
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 help

# Get help for specific command
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 help sendtoaddress
```

## Contributing

This is part of the Bitcoin Federation project. For development guidelines, see the project documentation.

## License

This project follows the same license as Bitcoin Core (MIT License).

---

**⚠️ Important**: This setup is for development and testing only. Never use these configurations or credentials in production environments. 