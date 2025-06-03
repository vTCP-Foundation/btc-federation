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

.PHONY: help btc-start btc-stop btc-reset btc-miner-start btc-miner-stop btc-status btc-logs

help:
	@echo "Bitcoin Fastnet Local Node Commands:"
	@echo "  btc-start        - Start Bitcoin node"
	@echo "  btc-stop         - Stop Bitcoin node"
	@echo "  btc-reset        - Restart with new genesis block"
	@echo "  btc-miner-start  - Start miner"
	@echo "  btc-miner-stop   - Stop miner"
	@echo "  btc-status       - Show node status"
	@echo "  btc-logs         - Show node logs"

# Start Bitcoin node
btc-start:
	@echo "Starting Bitcoin node..."
	@if pgrep -f "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
		echo "Bitcoin node is already running"; \
	else \
		$(BITCOIND) -conf=$(BITCOIN_CONF) -daemon; \
		echo "Waiting for node to start..."; \
		sleep 5; \
		$(BITCOIN_CLI) $(CLI_ARGS) getblockchaininfo > /dev/null && echo "Bitcoin node started successfully" || echo "Failed to start Bitcoin node"; \
	fi

# Stop Bitcoin node
btc-stop:
	@echo "Stopping Bitcoin node..."
	@if pgrep -f "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
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
	@echo "Starting fresh Bitcoin node..."
	@$(MAKE) btc-start

# Start miner (generate blocks continuously)
btc-miner-start:
	@echo "Starting miner..."
	@if [ -f .btc/miner.pid ] && kill -0 $$(cat .btc/miner.pid) 2>/dev/null; then \
		echo "Miner is already running (PID: $$(cat .btc/miner.pid))"; \
	else \
		$(BITCOIN_CLI) $(CLI_ARGS) createwallet "miner" 2>/dev/null || true; \
		ADDR=$$($(BITCOIN_CLI) $(CLI_ARGS) getnewaddress); \
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
	@if pgrep -f "bitcoind.*bitcoin-fastnet.conf" > /dev/null; then \
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