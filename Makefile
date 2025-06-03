# Bitcoin Fastnet Local Node Setup Makefile
# Task: btc-federation-1 - Bitcoin Fastnet Local Node Setup

# Bitcoin Core paths
PROJECT_ROOT = $(shell pwd)
BITCOIN_DIR = $(PROJECT_ROOT)/.btc/node
BITCOIN_DATA = $(PROJECT_ROOT)/.btc/data
BITCOIN_CONF = $(PROJECT_ROOT)/.btc/node/bitcoin-fastnet.conf
BITCOIND = $(BITCOIN_DIR)/bin/bitcoind
BITCOIN_CLI = $(BITCOIN_DIR)/bin/bitcoin-cli

# RPC connection settings
RPC_HOST = 127.0.0.1
RPC_PORT = 18443
RPC_USER = fastnet
RPC_PASSWORD = fastnet123
CLI_ARGS = -rpcconnect=$(RPC_HOST) -rpcport=$(RPC_PORT) -rpcuser=$(RPC_USER) -rpcpassword=$(RPC_PASSWORD)

# Mining settings
MINER_THREADS = 1
MINER_KEY_FILE = miner.key
MINER_ADDR_FILE = miner.address

.PHONY: help btc-setup btc-start btc-stop btc-reset btc-miner-start btc-miner-stop btc-status btc-logs

help:
	@echo "Bitcoin Fastnet Local Node Commands:"
	@echo "  btc-setup        - Initial setup of Bitcoin node"
	@echo "  btc-start        - Start Bitcoin node"
	@echo "  btc-stop         - Stop Bitcoin node"
	@echo "  btc-reset        - Restart with new genesis block"
	@echo "  btc-miner-start  - Start miner"
	@echo "  btc-miner-stop   - Stop miner"
	@echo "  btc-status       - Show node status"
	@echo "  btc-logs         - Show node logs"

# Initial setup of Bitcoin node
btc-setup:
	@echo "Setting up Bitcoin node..."
	@mkdir -p $(BITCOIN_DIR)
	@mkdir -p $(BITCOIN_DATA)
	@if [ ! -f $(BITCOIND) ]; then \
		echo "Downloading Bitcoin Core v29.0..."; \
		cd $(PROJECT_ROOT)/.btc && \
		wget -q https://bitcoincore.org/bin/bitcoin-core-29.0/bitcoin-29.0-x86_64-linux-gnu.tar.gz && \
		echo "Extracting Bitcoin Core..."; \
		tar -xzf bitcoin-29.0-x86_64-linux-gnu.tar.gz && \
		mv bitcoin-29.0/* node/ && \
		rm -rf bitcoin-29.0 bitcoin-29.0-x86_64-linux-gnu.tar.gz && \
		echo "Bitcoin Core v29.0 installed successfully"; \
	else \
		echo "Bitcoin Core binaries already exist at $(BITCOIN_DIR)"; \
	fi
	@if [ ! -f $(BITCOIN_CONF) ]; then \
		echo "Creating Bitcoin configuration file..."; \
		echo "# Bitcoin Core configuration for fastnet (regtest with fast blocks)" > $(BITCOIN_CONF); \
		echo "# Task: btc-federation-1 - Bitcoin Fastnet Local Node Setup" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Network settings" >> $(BITCOIN_CONF); \
		echo "regtest=1" >> $(BITCOIN_CONF); \
		echo "fallbackfee=0.0002" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# RPC settings - localhost only access" >> $(BITCOIN_CONF); \
		echo "server=1" >> $(BITCOIN_CONF); \
		echo "rpcuser=fastnet" >> $(BITCOIN_CONF); \
		echo "rpcpassword=fastnet123" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Memory and performance settings" >> $(BITCOIN_CONF); \
		echo "dbcache=100" >> $(BITCOIN_CONF); \
		echo "maxmempool=50" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Logging settings" >> $(BITCOIN_CONF); \
		echo "debug=0" >> $(BITCOIN_CONF); \
		echo "printtoconsole=1" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Disable external connections in regtest" >> $(BITCOIN_CONF); \
		echo "listen=0" >> $(BITCOIN_CONF); \
		echo "discover=0" >> $(BITCOIN_CONF); \
		echo "upnp=0" >> $(BITCOIN_CONF); \
		echo "natpmp=0" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Data directory (relative to project root)" >> $(BITCOIN_CONF); \
		echo "datadir=.btc/data" >> $(BITCOIN_CONF); \
		echo "" >> $(BITCOIN_CONF); \
		echo "# Regtest-specific settings" >> $(BITCOIN_CONF); \
		echo "[regtest]" >> $(BITCOIN_CONF); \
		echo "rpcbind=127.0.0.1:18443" >> $(BITCOIN_CONF); \
		echo "rpcallowip=127.0.0.1" >> $(BITCOIN_CONF); \
		echo "# Generate blocks every ~10 seconds" >> $(BITCOIN_CONF); \
		echo "blocktime=10" >> $(BITCOIN_CONF); \
		echo "# Block generation settings" >> $(BITCOIN_CONF); \
		echo "txconfirmtarget=1" >> $(BITCOIN_CONF); \
		echo "Configuration file created at $(BITCOIN_CONF)"; \
	else \
		echo "Configuration file already exists at $(BITCOIN_CONF)"; \
	fi
	@echo "Bitcoin node setup completed successfully"
	@echo "Run 'make btc-start' to start the node"

# Start Bitcoin node
btc-start:
	@echo "Starting Bitcoin node..."
	@if ps aux | grep -v grep | grep "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
		echo "Bitcoin node is already running"; \
	else \
		$(BITCOIND) -conf=$(BITCOIN_CONF) -deprecatedrpc=create_bdb -daemon; \
		echo "Waiting for node to start..."; \
		sleep 5; \
		$(BITCOIN_CLI) $(CLI_ARGS) getblockchaininfo > /dev/null && echo "Bitcoin node started successfully" || echo "Failed to start Bitcoin node"; \
	fi

# Stop Bitcoin node
btc-stop:
	@echo "Stopping Bitcoin node..."
	@if ps aux | grep -v grep | grep "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
		$(BITCOIN_CLI) $(CLI_ARGS) stop 2>/dev/null || true; \
		echo "Waiting for node to stop..."; \
		sleep 3; \
		pkill -f "bitcoind.*bitcoin-fastnet.conf" 2>/dev/null || true; \
		echo "Bitcoin node stopped"; \
	else \
		echo "Bitcoin node is not running"; \
	fi

# Reset blockchain data and restart
btc-reset: btc-stop
	@echo "Resetting blockchain data..."
	@rm -rf $(BITCOIN_DATA)/regtest
	@rm -f .btc/miner.pid .btc/miner.log
	@echo "Starting fresh Bitcoin node..."
	@$(MAKE) btc-start

# Start miner (generate blocks continuously)
btc-miner-start:
	@echo "Starting miner..."
	@if [ -f .btc/miner.pid ] && kill -0 $$(cat .btc/miner.pid) 2>/dev/null; then \
		echo "Miner is already running (PID: $$(cat .btc/miner.pid))"; \
	else \
		$(BITCOIN_CLI) $(CLI_ARGS) createwallet "miner" false false "" false false true 2>/dev/null || true; \
		if [ ! -f $(MINER_KEY_FILE) ]; then \
			echo "cMahea7zqjxrtgAbB7LSGbcQUr1uX1ojuat9jZodMN87JcbXMTcA" > $(MINER_KEY_FILE); \
			echo "bcrt1q0szhffryrh8s5ltv8ay4874gx3qs3q3d76chlq" > $(MINER_ADDR_FILE); \
		fi; \
		$(BITCOIN_CLI) $(CLI_ARGS) importprivkey "$$(cat $(MINER_KEY_FILE))" "fastnet-miner" false 2>/dev/null || true; \
		ADDR=$$(cat $(MINER_ADDR_FILE)); \
		echo "Mining to address: $$ADDR"; \
		nohup bash -c 'while true; do $(BITCOIN_CLI) $(CLI_ARGS) generatetoaddress 1 '$$ADDR' > /dev/null 2>&1; sleep 10; done' > .btc/miner.log 2>&1 & \
		echo $$! > .btc/miner.pid; \
		echo "Miner started (PID: $$(cat .btc/miner.pid))"; \
	fi

# Stop miner
btc-miner-stop:
	@echo "Stopping miner..."
	@if [ -f .btc/miner.pid ]; then \
		PID=$$(cat .btc/miner.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID; \
			rm -f .btc/miner.pid; \
			echo "Miner stopped"; \
		else \
			echo "Miner process not found, cleaning up PID file"; \
			rm -f .btc/miner.pid; \
		fi; \
	else \
		pkill -f "generatetoaddress" 2>/dev/null && echo "Miner stopped" || echo "Miner is not running"; \
	fi

# Show node status
btc-status:
	@echo "Bitcoin Node Status:"
	@if ps aux | grep -v grep | grep "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
		echo "Node: Running"; \
		$(BITCOIN_CLI) $(CLI_ARGS) getblockchaininfo 2>/dev/null | grep -E '"blocks"|"difficulty"' || echo "Node not responding"; \
	else \
		echo "Node: Stopped"; \
	fi
	@echo "Miner Status:"
	@if [ -f .btc/miner.pid ] && kill -0 $$(cat .btc/miner.pid) 2>/dev/null; then \
		echo "Miner: Running (PID: $$(cat .btc/miner.pid))"; \
	else \
		echo "Miner: Stopped"; \
	fi

# Show recent logs
btc-logs:
	@echo "Bitcoin Node Logs:"
	@if [ -f $(BITCOIN_DATA)/regtest/debug.log ]; then \
		tail -20 $(BITCOIN_DATA)/regtest/debug.log; \
	else \
		echo "No log file found"; \
	fi
	@echo ""
	@echo "Miner Logs:"
	@if [ -f .btc/miner.log ]; then \
		tail -10 .btc/miner.log; \
	else \
		echo "No miner log file found"; \
	fi 