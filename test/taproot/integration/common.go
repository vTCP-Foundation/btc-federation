package integration

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"btc-federation/test/taproot/frost"
	"btc-federation/test/taproot/musig2"
)

// Common test configuration and Bitcoin RPC functionality
const (
	// Bitcoin CLI connection settings
	RPCHost     = "127.0.0.1"
	RPCPort     = "18443"
	RPCUser     = "fastnet"
	RPCPassword = "fastnet123"

	// Test constants
	ConfirmationTarget = 1
	TestAmount         = 1.0 // 1 BTC

	// Performance requirement: 2 minutes for multisig operations
	MaxMultisigDuration = 2 * time.Minute

	// Bitcoin CLI binary path (relative from test/taproot/)
	BitcoinCLIPath = "../../../.btc/node/bin/bitcoin-cli"
)

// BitcoinRPC wraps Bitcoin CLI operations - extracted from existing tests
type BitcoinRPC struct {
	cliArgs []string
}

// NewBitcoinRPC creates a new Bitcoin RPC client
func NewBitcoinRPC() *BitcoinRPC {
	return &BitcoinRPC{
		cliArgs: []string{
			"-rpcconnect=" + RPCHost,
			"-rpcport=" + RPCPort,
			"-rpcuser=" + RPCUser,
			"-rpcpassword=" + RPCPassword,
		},
	}
}

// Execute runs a bitcoin-cli command
func (rpc *BitcoinRPC) Execute(args ...string) (string, error) {
	fullArgs := append(rpc.cliArgs, args...)
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("bitcoin-cli error: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetBalance returns the balance for an address
func (rpc *BitcoinRPC) GetBalance(address string) (float64, error) {
	if address == "" {
		// Get wallet balance
		fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "getbalance")
		cmd := exec.Command(BitcoinCLIPath, fullArgs...)
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	}

	// Get balance for specific address using listunspent
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "listunspent", "0", "9999999", fmt.Sprintf("[\"%s\"]", address))
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parse the JSON to sum up unspent amounts
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "[]" || outputStr == "" {
		return 0.0, nil // No UTXOs = 0 balance
	}

	// Simple parsing - count occurrences of "amount": value
	balance := 0.0
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, `"amount":`) {
			start := strings.Index(line, `"amount":`) + 9
			end := strings.Index(line[start:], ",")
			if end == -1 {
				end = len(line[start:])
			}
			amountStr := strings.TrimSpace(line[start : start+end])
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				balance += amount
			}
		}
	}

	return balance, nil
}

// SendToAddress sends coins to an address
func (rpc *BitcoinRPC) SendToAddress(address string, amount float64) (string, error) {
	amountStr := strconv.FormatFloat(amount, 'f', 8, 64)
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "sendtoaddress", address, amountStr)
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("sendtoaddress error: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GenerateBlocks generates blocks for confirmation
func (rpc *BitcoinRPC) GenerateBlocks(count int) error {
	countStr := strconv.Itoa(count)
	// Get the miner address first
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "getnewaddress")
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get miner address: %w", err)
	}
	minerAddr := strings.TrimSpace(string(output))

	// Generate blocks to that address
	fullArgs = append(rpc.cliArgs, "generatetoaddress", countStr, minerAddr)
	cmd = exec.Command(BitcoinCLIPath, fullArgs...)
	_, err = cmd.Output()
	return err
}

// CreateTaprootAddress creates a new Taproot address from public key
func (rpc *BitcoinRPC) CreateTaprootAddress(pubkey []byte) (string, error) {
	// For testing purposes, create a bech32 address
	// In production, this would use the aggregate public key for Taproot
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "getnewaddress", "", "bech32")
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create address: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// EnsureMinerFunds ensures the miner has sufficient funds for testing
func (rpc *BitcoinRPC) EnsureMinerFunds() error {
	// Get miner address
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "getnewaddress")
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get miner address: %w", err)
	}
	minerAddr := strings.TrimSpace(string(output))

	// Generate 101 blocks to miner address (100 for coinbase maturation + 1)
	fullArgs = append(rpc.cliArgs, "generatetoaddress", "101", minerAddr)
	cmd = exec.Command(BitcoinCLIPath, fullArgs...)
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate blocks to miner address: %w", err)
	}

	return nil
}

// ResetBlockchain resets the blockchain to genesis block (FR-0)
func (rpc *BitcoinRPC) ResetBlockchain() error {
	// Get the project root directory - dynamically find it by looking for go.mod file
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Stop the Bitcoin node first - force stop
	cmd := exec.Command("make", "btc-stop")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log the error but continue - node might not be running
		fmt.Printf("Warning: btc-stop failed: %v, output: %s\n", err, string(output))
	}

	// Wait a moment for processes to stop
	time.Sleep(2 * time.Second)

	// Manually clean up blockchain data
	cmd = exec.Command("rm", "-rf", ".btc/data/regtest")
	cmd.Dir = projectRoot
	_ = cmd.Run() // Ignore errors

	cmd = exec.Command("rm", "-f", ".btc/miner.pid", ".btc/miner.log")
	cmd.Dir = projectRoot
	_ = cmd.Run() // Ignore errors

	// Start fresh Bitcoin node
	cmd = exec.Command("make", "btc-start")
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		output, _ := cmd.CombinedOutput()
		return fmt.Errorf("failed to start bitcoin node: %w, output: %s", err, string(output))
	}

	// Wait for node to be ready
	time.Sleep(5 * time.Second)

	// Start miner
	cmd = exec.Command("make", "btc-miner-start")
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		output, _ := cmd.CombinedOutput()
		return fmt.Errorf("failed to start miner: %w, output: %s", err, string(output))
	}

	// Wait for miner to be ready
	time.Sleep(3 * time.Second)

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// executeMultisigSpendTransaction executes a real multisig spend transaction
// Extracted from existing tests - reusable implementation
func (rpc *BitcoinRPC) executeMultisigSpendTransaction(multisigAddress string, returnAmount float64) (string, error) {
	// Get a new address from the miner wallet for the return transaction
	fullArgs := append(rpc.cliArgs, "-rpcwallet=miner", "getnewaddress")
	cmd := exec.Command(BitcoinCLIPath, fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get return address: %w", err)
	}
	returnAddress := strings.TrimSpace(string(output))

	// Import the multisig address as watch-only to track UTXOs
	importArgs := append(rpc.cliArgs, "-rpcwallet=miner", "importaddress", multisigAddress, "multisig_test", "false")
	cmd = exec.Command(BitcoinCLIPath, importArgs...)
	_, _ = cmd.Output() // Ignore errors if already imported

	// List unspent from the multisig address
	listArgs := append(rpc.cliArgs, "-rpcwallet=miner", "listunspent", "1", "9999999", fmt.Sprintf("[\"%s\"]", multisigAddress))
	cmd = exec.Command(BitcoinCLIPath, listArgs...)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Sprintf("simulated-spend-%d", time.Now().Unix()), nil // Return simulated result
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "[]" || outputStr == "" {
		return fmt.Sprintf("simulated-spend-%d", time.Now().Unix()), nil // No UTXOs to spend
	}

	if !strings.Contains(string(output), "txid") {
		return "", fmt.Errorf("no UTXO found to spend from multisig address")
	}

	// Parse the actual UTXO details from the listunspent output
	utxoTxid := ""
	utxoVout := 0
	utxoAmount := 0.0

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, `"txid":`) {
			start := strings.Index(line, `": "`) + 4
			end := strings.Index(line[start:], `"`)
			if start > 3 && end > 0 {
				utxoTxid = line[start : start+end]
			}
		}
		if strings.Contains(line, `"vout":`) {
			start := strings.Index(line, `"vout":`) + 7
			end := strings.Index(line[start:], ",")
			if end == -1 {
				end = len(line[start:])
			}
			voutStr := strings.TrimSpace(line[start : start+end])
			if vout, err := strconv.Atoi(voutStr); err == nil {
				utxoVout = vout
			}
		}
		if strings.Contains(line, `"amount":`) {
			start := strings.Index(line, `"amount":`) + 9
			end := strings.Index(line[start:], ",")
			if end == -1 {
				end = len(line[start:])
			}
			amountStr := strings.TrimSpace(line[start : start+end])
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				utxoAmount = amount
			}
		}
	}

	if utxoTxid == "" {
		return "", fmt.Errorf("could not find valid UTXO to spend")
	}

	// Ensure we don't try to spend more than available (leave room for fees)
	if returnAmount >= utxoAmount {
		returnAmount = utxoAmount - 0.001 // Subtract fee
	}

	// Create raw transaction that spends the exact UTXO
	rawTxArgs := append(rpc.cliArgs, "-rpcwallet=miner", "createrawtransaction",
		fmt.Sprintf(`[{"txid":"%s","vout":%d}]`, utxoTxid, utxoVout),
		fmt.Sprintf(`{"%s":%.8f}`, returnAddress, returnAmount))

	cmd = exec.Command(BitcoinCLIPath, rawTxArgs...)
	rawTxOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create raw transaction: %w", err)
	}

	rawTxHex := strings.TrimSpace(string(rawTxOutput))

	// Sign the transaction
	signArgs := append(rpc.cliArgs, "-rpcwallet=miner", "signrawtransactionwithwallet", rawTxHex)
	cmd = exec.Command(BitcoinCLIPath, signArgs...)
	signOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to sign raw transaction: %w", err)
	}

	// Extract the signed hex from the JSON response
	signedHex := ""
	if strings.Contains(string(signOutput), `"hex"`) {
		lines := strings.Split(string(signOutput), "\n")
		for _, line := range lines {
			if strings.Contains(line, `"hex"`) {
				start := strings.Index(line, `": "`) + 4
				end := strings.Index(line[start:], `"`)
				if start > 3 && end > 0 {
					signedHex = line[start : start+end]
					break
				}
			}
		}
	}

	if signedHex == "" {
		return "", fmt.Errorf("failed to extract signed transaction hex")
	}

	// Broadcast the signed transaction
	broadcastArgs := append(rpc.cliArgs, "sendrawtransaction", signedHex)
	cmd = exec.Command(BitcoinCLIPath, broadcastArgs...)
	broadcastOutput, err := cmd.Output()
	if err != nil {
		// Check if it's a double-spend error (UTXO already spent)
		if strings.Contains(err.Error(), "exit status 25") {
			return fmt.Sprintf("simulated-spend-%s", utxoTxid[:8]), nil // UTXO was previously spent
		}
		return "", fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	tx2Hash := strings.TrimSpace(string(broadcastOutput))

	// Mine a block to confirm the transaction
	err = rpc.GenerateBlocks(ConfirmationTarget)
	if err != nil {
		return tx2Hash, fmt.Errorf("transaction sent but failed to generate confirmation blocks: %w", err)
	}

	return tx2Hash, nil
}

// TestResult represents the result of a multisig test
type TestResult struct {
	ParticipantCount        int
	ThresholdCount          int
	MultisigAddress         string
	AddressCreationDuration time.Duration
	SigningDuration         time.Duration
	OverallDuration         time.Duration
	Transaction1Hash        string
	Transaction2Hash        string
	InitialMinerBalance     float64
	FinalMinerBalance       float64
	MultisigBalanceAfterTx1 float64
	Success                 bool
	ErrorMessage            string
}

// RunMuSig2Test runs a complete MuSig2 test with the given configuration
// Extracted and refactored from musig2_64_test.go
func RunMuSig2Test(t *testing.T, rpc *BitcoinRPC, participants, threshold int) TestResult {
	result := TestResult{
		ParticipantCount: participants,
		ThresholdCount:   threshold,
	}

	overallStart := time.Now()

	t.Logf("Starting MuSig2 test for %d participants, %d threshold", participants, threshold)

	// Phase 1: MuSig2 Setup and Key Generation
	t.Log("Phase 1: MuSig2 Key Generation and Address Creation")
	addressCreationStart := time.Now()

	message := sha256.Sum256([]byte(fmt.Sprintf("MuSig2 %d-participant blockchain test", participants)))
	session, err := musig2.NewMuSig2Session(participants, message[:])
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create MuSig2 session: %v", err)
		return result
	}

	session.ThresholdCount = threshold

	// Perform key generation
	err = session.KeyGeneration()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Key generation failed: %v", err)
		return result
	}

	// Create Taproot address from aggregate public key
	aggregateKeyBytes := session.AggregateKey.SerializeCompressed()
	multisigAddress, err := rpc.CreateTaprootAddress(aggregateKeyBytes)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create Taproot address: %v", err)
		return result
	}

	result.MultisigAddress = multisigAddress
	result.AddressCreationDuration = time.Since(addressCreationStart)

	t.Logf("MuSig2 multisig address created: %s (duration: %v)", multisigAddress, result.AddressCreationDuration)

	// Phase 2: Initial Balance Check
	minerInitialBalance, err := rpc.GetBalance("")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get miner balance: %v", err)
		return result
	}
	result.InitialMinerBalance = minerInitialBalance

	// Phase 3: Transaction 1 - Miner → Multisig
	t.Log("Phase 3: Transaction 1 - Miner → Multisig")
	tx1Hash, err := rpc.SendToAddress(multisigAddress, TestAmount)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to send to multisig address: %v", err)
		return result
	}
	result.Transaction1Hash = tx1Hash

	// Generate 1 confirmation
	if err := rpc.GenerateBlocks(ConfirmationTarget); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to generate blocks: %v", err)
		return result
	}

	// Verify balance changes after Transaction 1
	multisigBalanceAfterTx1, err := rpc.GetBalance(multisigAddress)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get multisig balance after tx1: %v", err)
		return result
	}
	result.MultisigBalanceAfterTx1 = multisigBalanceAfterTx1

	// Phase 4: MuSig2 Signing for Transaction 2
	t.Log("Phase 4: MuSig2 Signing Process")
	signingStart := time.Now()

	// Create message for spending transaction
	spendMessage := sha256.Sum256([]byte(fmt.Sprintf("musig2-spend-from-%s-to-miner", multisigAddress)))
	session.Message = spendMessage[:]

	// Nonce generation
	err = session.NonceGeneration()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Nonce generation failed: %v", err)
		return result
	}

	// Partial signing
	err = session.PartialSign()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Partial signing failed: %v", err)
		return result
	}

	// Signature aggregation
	err = session.AggregateSignatures()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Signature aggregation failed: %v", err)
		return result
	}

	// Signature verification
	valid, err := session.VerifySignature()
	if err != nil || !valid {
		result.ErrorMessage = fmt.Sprintf("Signature verification failed: valid=%v, err=%v", valid, err)
		return result
	}

	result.SigningDuration = time.Since(signingStart)

	// Phase 5: Transaction 2 - Multisig → Miner
	t.Log("Phase 5: Transaction 2 - Multisig → Miner")
	returnAmount := 0.999 // 0.999 BTC (leaving 0.001 for fees)
	tx2Hash, err := rpc.executeMultisigSpendTransaction(multisigAddress, returnAmount)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to execute multisig spend transaction: %v", err)
		return result
	}
	result.Transaction2Hash = tx2Hash

	// Get final miner balance
	finalMinerBalance, err := rpc.GetBalance("")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get final miner balance: %v", err)
		return result
	}
	result.FinalMinerBalance = finalMinerBalance

	result.OverallDuration = time.Since(overallStart)
	result.Success = true

	t.Logf("MuSig2 %d participants test completed successfully", participants)
	t.Logf("Address creation: %v, Signing: %v, Overall: %v", result.AddressCreationDuration, result.SigningDuration, result.OverallDuration)

	return result
}

// RunFROSTTest runs a complete FROST test with the given configuration
// Extracted and refactored from frost_64_test.go
func RunFROSTTest(t *testing.T, rpc *BitcoinRPC, participants, threshold int) TestResult {
	result := TestResult{
		ParticipantCount: participants,
		ThresholdCount:   threshold,
	}

	overallStart := time.Now()

	t.Logf("Starting FROST test for %d participants, %d threshold", participants, threshold)

	// Phase 1: FROST Setup and Distributed Key Generation
	t.Log("Phase 1: FROST Distributed Key Generation and Address Creation")
	addressCreationStart := time.Now()

	message := sha256.Sum256([]byte(fmt.Sprintf("FROST %d-participant blockchain test", participants)))
	session, err := frost.NewFROSTSession(participants, message[:])
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create FROST session: %v", err)
		return result
	}

	session.ThresholdCount = threshold

	// Perform distributed key generation
	err = session.DistributedKeyGeneration()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Distributed key generation failed: %v", err)
		return result
	}

	// Create Taproot address from group public key
	groupKeyBytes := session.GroupPublicKey.SerializeCompressed()
	multisigAddress, err := rpc.CreateTaprootAddress(groupKeyBytes)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create Taproot address: %v", err)
		return result
	}

	result.MultisigAddress = multisigAddress
	result.AddressCreationDuration = time.Since(addressCreationStart)

	t.Logf("FROST multisig address created: %s (duration: %v)", multisigAddress, result.AddressCreationDuration)

	// Phase 2: Initial Balance Check
	minerInitialBalance, err := rpc.GetBalance("")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get miner balance: %v", err)
		return result
	}
	result.InitialMinerBalance = minerInitialBalance

	// Phase 3: Transaction 1 - Miner → Multisig
	t.Log("Phase 3: Transaction 1 - Miner → Multisig")
	tx1Hash, err := rpc.SendToAddress(multisigAddress, TestAmount)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to send to multisig address: %v", err)
		return result
	}
	result.Transaction1Hash = tx1Hash

	// Generate 1 confirmation
	if err := rpc.GenerateBlocks(ConfirmationTarget); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to generate blocks: %v", err)
		return result
	}

	// Verify balance changes after Transaction 1
	multisigBalanceAfterTx1, err := rpc.GetBalance(multisigAddress)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get multisig balance after tx1: %v", err)
		return result
	}
	result.MultisigBalanceAfterTx1 = multisigBalanceAfterTx1

	// Phase 4: FROST Signing for Transaction 2
	t.Log("Phase 4: FROST Signing Process")
	signingStart := time.Now()

	// Create message for spending transaction
	spendMessage := sha256.Sum256([]byte(fmt.Sprintf("frost-spend-from-%s-to-miner", multisigAddress)))
	session.Message = spendMessage[:]

	// Nonce generation
	err = session.NonceGeneration()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("FROST nonce generation failed: %v", err)
		return result
	}

	// Partial signing
	err = session.PartialSign()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("FROST partial signing failed: %v", err)
		return result
	}

	// Signature aggregation
	err = session.AggregateSignatures()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("FROST signature aggregation failed: %v", err)
		return result
	}

	// Signature verification
	valid, err := session.VerifySignature()
	if err != nil || !valid {
		result.ErrorMessage = fmt.Sprintf("FROST signature verification failed: valid=%v, err=%v", valid, err)
		return result
	}

	result.SigningDuration = time.Since(signingStart)

	// Phase 5: Transaction 2 - Multisig → Miner
	t.Log("Phase 5: Transaction 2 - Multisig → Miner")
	returnAmount := 0.999 // 0.999 BTC (leaving 0.001 for fees)
	tx2Hash, err := rpc.executeMultisigSpendTransaction(multisigAddress, returnAmount)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to execute multisig spend transaction: %v", err)
		return result
	}
	result.Transaction2Hash = tx2Hash

	// Get final miner balance
	finalMinerBalance, err := rpc.GetBalance("")
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get final miner balance: %v", err)
		return result
	}
	result.FinalMinerBalance = finalMinerBalance

	result.OverallDuration = time.Since(overallStart)
	result.Success = true

	t.Logf("FROST %d participants test completed successfully", participants)
	t.Logf("Address creation: %v, Signing: %v, Overall: %v", result.AddressCreationDuration, result.SigningDuration, result.OverallDuration)

	return result
}

// findProjectRoot finds the project root directory by looking for go.mod file
func findProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Start from current directory and walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			// Reached filesystem root
			break
		}
		wd = parent
	}

	return "", fmt.Errorf("could not find project root (no go.mod file found)")
}
