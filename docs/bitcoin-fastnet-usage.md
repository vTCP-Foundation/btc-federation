# Bitcoin Fastnet Usage Guide

This guide provides instructions for using the Bitcoin Fastnet local regtest node for development purposes.

## Getting Started

### Starting the Node

```bash
make btc-start
```

This will start the Bitcoin Core node in regtest mode with fast block generation (~10 seconds).

### Starting the Miner

```bash
make btc-miner-start
```

This will start the automatic block generation process, creating new blocks every 10 seconds.

## Wallet Operations

### Creating a Wallet

Create a new wallet for your development work:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 createwallet "development"
```

**Note**: For operations requiring private key imports (like `importprivkey`), you need to create a legacy wallet:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 createwallet "development" false false "" false false true
```

Load an existing wallet:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 loadwallet "development"
```

List all wallets:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 listwallets
```

### Getting Addresses

Generate a new Bitcoin address:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getnewaddress
```

Generate a specific address type (legacy, p2sh-segwit, bech32):

```bash
# Legacy address
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getnewaddress "" "legacy"

# SegWit address (bech32)
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getnewaddress "" "bech32"
```

### Balance Checking

Check wallet balance:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getbalance
```

Get detailed balance information:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getwalletinfo
```

List all transactions:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 listtransactions
```

Check balance for a specific address:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getaddressinfo "YOUR_ADDRESS"
```

## Transaction Operations

### Sending Transactions

Send Bitcoin to an address:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 sendtoaddress "RECIPIENT_ADDRESS" 1.5
```

Send with a comment:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 sendtoaddress "RECIPIENT_ADDRESS" 1.5 "payment comment" "recipient comment"
```

### Creating Raw Transactions

List unspent outputs:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 listunspent
```

Create a raw transaction:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 createrawtransaction '[{"txid":"TXID","vout":0}]' '{"RECIPIENT_ADDRESS":1.5}'
```

Sign the raw transaction:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 signrawtransactionwithwallet "RAW_TX_HEX"
```

Send the raw transaction:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 sendrawtransaction "SIGNED_TX_HEX"
```

## Transaction Status Verification

### Checking Transaction Status

Get transaction details:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 gettransaction "TXID"
```

Get raw transaction information:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getrawtransaction "TXID" true
```

Check if transaction is in mempool:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getmempoolentry "TXID"
```

Get mempool information:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getmempoolinfo
```

### Block Information

Get current block height:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getblockcount
```

Get blockchain information:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getblockchaininfo
```

Get block hash by height:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getblockhash 100
```

Get block details:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 getblock "BLOCK_HASH"
```

## Development Utilities

### Generating Test Bitcoin

Generate blocks to a specific address (useful for testing):

```bash
# Generate 100 blocks to get mature coinbase transactions
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 generatetoaddress 100 "YOUR_ADDRESS"
```

### Node Management

Check node status:

```bash
make btc-status
```

View logs:

```bash
make btc-logs
```

Stop the node:

```bash
make btc-stop
```

Reset the blockchain (start fresh):

```bash
make btc-reset
```

Stop the miner:

```bash
make btc-miner-stop
```

## Troubleshooting

### Common Issues

1. **Node won't start**: Check if port 18443 is already in use
2. **RPC connection failed**: Ensure the node is running and RPC credentials are correct
3. **Transaction not confirming**: Check if the miner is running
4. **Insufficient funds**: Generate blocks to your address first

### Getting Help

View available RPC commands:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 help
```

Get help for a specific command:

```bash
.btc/node/bin/bitcoin-cli -rpcconnect=127.0.0.1 -rpcport=18443 -rpcuser=fastnet -rpcpassword=fastnet123 help sendtoaddress
```

## Legacy Wallet Support

This Bitcoin Fastnet setup uses deprecated BDB wallet functionality to support private key import operations required for mining. The node is started with the `-deprecatedrpc=create_bdb` flag to enable legacy wallet creation.

### Why Legacy Wallets?

- **Private Key Import**: The `importprivkey` command only works with legacy (BDB) wallets
- **Mining Setup**: The miner requires importing a pre-generated private key for block rewards
- **Development Testing**: Legacy wallets provide full control over keys for testing scenarios

### Legacy Wallet Parameters

When creating a legacy wallet, use these parameters:

```bash
createwallet "wallet_name" false false "" false false true
```

Parameters explanation:
- `wallet_name`: Name of the wallet
- `false`: Don't disable private keys
- `false`: Don't create a blank wallet
- `""`: No passphrase
- `false`: Don't avoid address reuse
- `false`: Don't create descriptors-only wallet
- `true`: Create legacy (BDB) wallet

## Security Note

This setup is for development purposes only. The RPC credentials, deprecated wallet functionality, and setup are not secure and should never be used in production environments. 