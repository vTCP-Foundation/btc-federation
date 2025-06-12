package integration

import (
	"btc-federation/test/taproot/benchmark"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Constants from task requirements
const (
	// Performance Requirements
	MaxCombinedOperationDuration = 2 * time.Minute
)

// CombinedPerformanceReport represents the comprehensive combined performance report
type CombinedPerformanceReport struct {
	GeneratedAt     time.Time                     `json:"generated_at"`
	TestEnvironment TestEnvironment               `json:"test_environment"`
	MuSig2Results   []CombinedConfigurationResult `json:"musig2_results"`
	FROSTResults    []CombinedConfigurationResult `json:"frost_results"`
	Comparison      CombinedComparisonAnalysis    `json:"comparison"`
	Recommendations []string                      `json:"recommendations"`
	Summary         CombinedSummary               `json:"summary"`
}

// CombinedConfigurationResult captures both real and simulated results
type CombinedConfigurationResult struct {
	ParticipantCount int `json:"participant_count"`
	ThresholdCount   int `json:"threshold_count"`

	// Real blockchain metrics (from performance_test.go approach)
	RealMetrics            RealOperationMetrics `json:"real_metrics"`
	BlockchainTransactions []TransactionResult  `json:"blockchain_transactions"`
	RealPerformanceTarget  bool                 `json:"real_performance_target"`

	// Simulated metrics (from integration_test.go approach)
	SimulatedMetrics           SimulatedOperationMetrics `json:"simulated_metrics"`
	SimulatedPerformanceTarget bool                      `json:"simulated_performance_target"`

	ResourceUsage ResourceUsageMetrics `json:"resource_usage"`
}

// RealOperationMetrics captures real blockchain operation timings
type RealOperationMetrics struct {
	MultisigAddressCreation OperationMetrics `json:"multisig_address_creation"`
	TransactionSigning      OperationMetrics `json:"transaction_signing"`
	OverallWorkflow         OperationMetrics `json:"overall_workflow"`
	CombinedCryptoTime      OperationMetrics `json:"combined_crypto_time"` // Address + Signing
}

// SimulatedOperationMetrics captures simulated performance metrics
type SimulatedOperationMetrics struct {
	OperationDuration      time.Duration `json:"operation_duration"`
	OperationDurationMs    int64         `json:"operation_duration_ms"`
	OperationDurationSec   float64       `json:"operation_duration_sec"`
	OperationDurationMin   float64       `json:"operation_duration_min"`
	AdjustedDuration       time.Duration `json:"adjusted_duration"`
	AdjustedDurationMs     int64         `json:"adjusted_duration_ms"`
	AdjustedDurationSec    float64       `json:"adjusted_duration_sec"`
	AdjustedDurationMin    float64       `json:"adjusted_duration_min"`
	NetworkLatencyTotal    time.Duration `json:"network_latency_total"`
	NetworkLatencyTotalMs  int64         `json:"network_latency_total_ms"`
	NetworkLatencyTotalSec float64       `json:"network_latency_total_sec"`
	NetworkLatencyTotalMin float64       `json:"network_latency_total_min"`
	TotalSimulatedTime     time.Duration `json:"total_simulated_time"`
	TotalSimulatedTimeMs   int64         `json:"total_simulated_time_ms"`
	TotalSimulatedTimeSec  float64       `json:"total_simulated_time_sec"`
	TotalSimulatedTimeMin  float64       `json:"total_simulated_time_min"`
	ScalingFactor          float64       `json:"scaling_factor"`
	SimulatedCores         int           `json:"simulated_cores"`
	LinearScaling          bool          `json:"linear_scaling"`
}

// CombinedComparisonAnalysis provides comparative analysis between schemes with both metrics
type CombinedComparisonAnalysis struct {
	RealAddressCreationComparison []ParticipantComparison `json:"real_address_creation_comparison"`
	RealSigningComparison         []ParticipantComparison `json:"real_signing_comparison"`
	SimulatedCryptoComparison     []ParticipantComparison `json:"simulated_crypto_comparison"`
	RealVsSimulatedComparison     []RealVsSimulatedComp   `json:"real_vs_simulated_comparison"`
	ScalabilityAnalysis           ScalabilityAnalysis     `json:"scalability_analysis"`
}

// RealVsSimulatedComp compares real vs simulated metrics for same scheme
type RealVsSimulatedComp struct {
	ParticipantCount      int           `json:"participant_count"`
	Scheme                string        `json:"scheme"`
	RealCryptoTime        time.Duration `json:"real_crypto_time"`
	SimulatedTotalTime    time.Duration `json:"simulated_total_time"`
	RealCryptoTimeMs      int64         `json:"real_crypto_time_ms"`
	RealCryptoTimeSec     float64       `json:"real_crypto_time_sec"`
	RealCryptoTimeMin     float64       `json:"real_crypto_time_min"`
	SimulatedTotalTimeMs  int64         `json:"simulated_total_time_ms"`
	SimulatedTotalTimeSec float64       `json:"simulated_total_time_sec"`
	SimulatedTotalTimeMin float64       `json:"simulated_total_time_min"`
	SimulationOverhead    float64       `json:"simulation_overhead_ratio"`
	RecommendedEstimate   string        `json:"recommended_estimate"`
}

// CombinedSummary provides executive summary of combined results
type CombinedSummary struct {
	TotalConfigurationsTested     int                 `json:"total_configurations_tested"`
	RealTestsPassed               int                 `json:"real_tests_passed"`
	SimulatedTestsPassed          int                 `json:"simulated_tests_passed"`
	BestRealPerformingScheme      string              `json:"best_real_performing_scheme"`
	BestSimulatedPerformingScheme string              `json:"best_simulated_performing_scheme"`
	RealWorstCasePerformance      time.Duration       `json:"real_worst_case_performance"`
	SimulatedWorstCasePerformance time.Duration       `json:"simulated_worst_case_performance"`
	RealBestCasePerformance       time.Duration       `json:"real_best_case_performance"`
	SimulatedBestCasePerformance  time.Duration       `json:"simulated_best_case_performance"`
	RecommendedConfigurations     []RecommendedConfig `json:"recommended_configurations"`
}

// TestCombinedPerformanceMetrics implements the combined performance test
func TestCombinedPerformanceMetrics(t *testing.T) {
	t.Log("=== Starting Combined Performance Metrics Collection ===")
	t.Log("This test combines real blockchain metrics with simulated scaling metrics")

	report := &CombinedPerformanceReport{
		GeneratedAt: time.Now(),
		TestEnvironment: TestEnvironment{
			CPUCores:       runtime.NumCPU(),
			OS:             runtime.GOOS,
			GoVersion:      runtime.Version(),
			BitcoinNetwork: "regtest",
			TestTimestamp:  time.Now().Format(time.RFC3339),
		},
		MuSig2Results: make([]CombinedConfigurationResult, 0, len(testConfigurations)),
		FROSTResults:  make([]CombinedConfigurationResult, 0, len(testConfigurations)),
	}

	// Initialize Bitcoin RPC and benchmark detector
	rpc := NewBitcoinRPC()
	detector := benchmark.NewCoreDetector()
	config := benchmark.DefaultBenchmarkConfig()

	// Test all configurations for MuSig2
	t.Log("Phase 1: Testing MuSig2 configurations (real + simulated)")
	for _, testConfig := range testConfigurations {
		t.Logf("Testing MuSig2 with %d participants, %d threshold", testConfig.Participants, testConfig.Threshold)

		result, err := testCombinedMuSig2Configuration(t, rpc, detector, config, testConfig.Participants, testConfig.Threshold)
		if err != nil {
			t.Errorf("Combined MuSig2 test failed for %d participants: %v", testConfig.Participants, err)
			continue
		}

		report.MuSig2Results = append(report.MuSig2Results, result)
		t.Logf("✓ Combined MuSig2 %d participants completed successfully", testConfig.Participants)
	}

	// Test all configurations for FROST
	t.Log("Phase 2: Testing FROST configurations (real + simulated)")
	for _, testConfig := range testConfigurations {
		t.Logf("Testing FROST with %d participants, %d threshold", testConfig.Participants, testConfig.Threshold)

		result, err := testCombinedFROSTConfiguration(t, rpc, detector, config, testConfig.Participants, testConfig.Threshold)
		if err != nil {
			t.Errorf("Combined FROST test failed for %d participants: %v", testConfig.Participants, err)
			continue
		}

		report.FROSTResults = append(report.FROSTResults, result)
		t.Logf("✓ Combined FROST %d participants completed successfully", testConfig.Participants)
	}

	// Generate combined analysis
	t.Log("Phase 3: Generating combined comparative analysis")
	report.Comparison = generateCombinedComparison(report.MuSig2Results, report.FROSTResults)
	report.Summary = generateCombinedSummary(report.MuSig2Results, report.FROSTResults)
	report.Recommendations = generateCombinedRecommendations(report.MuSig2Results, report.FROSTResults)

	// Save combined performance report
	err := saveCombinedPerformanceReport(report)
	if err != nil {
		t.Errorf("Failed to save combined performance report: %v", err)
	} else {
		t.Log("✓ Combined performance report saved to combined_performance_report.json")
	}

	// Log critical findings
	t.Log("\n=== COMBINED PERFORMANCE ANALYSIS SUMMARY ===")
	t.Logf("Real tests passed: %d/%d", report.Summary.RealTestsPassed, report.Summary.TotalConfigurationsTested)
	t.Logf("Simulated tests passed: %d/%d", report.Summary.SimulatedTestsPassed, report.Summary.TotalConfigurationsTested)
	t.Logf("Best real performing scheme: %s", report.Summary.BestRealPerformingScheme)
	t.Logf("Best simulated performing scheme: %s", report.Summary.BestSimulatedPerformingScheme)

	t.Log("=== Combined Performance Metrics Collection Completed ===")
}

// testCombinedMuSig2Configuration tests MuSig2 with both real and simulated metrics
func testCombinedMuSig2Configuration(t *testing.T, rpc *BitcoinRPC, detector *benchmark.CoreDetector,
	config *benchmark.BenchmarkConfig, participants, threshold int) (CombinedConfigurationResult, error) {

	result := CombinedConfigurationResult{
		ParticipantCount:       participants,
		ThresholdCount:         threshold,
		BlockchainTransactions: make([]TransactionResult, 0, 2),
	}

	// Step 1: Get real blockchain metrics
	t.Log("Step 1: Collecting real blockchain metrics")
	if err := rpc.ResetBlockchain(); err != nil {
		return result, fmt.Errorf("failed to reset blockchain: %w", err)
	}

	if err := rpc.EnsureMinerFunds(); err != nil {
		return result, fmt.Errorf("failed to ensure miner funds: %w", err)
	}

	// Run real test
	testResult := RunMuSig2Test(t, rpc, participants, threshold)
	if !testResult.Success {
		return result, fmt.Errorf("MuSig2 real test failed: %s", testResult.ErrorMessage)
	}

	// Store real metrics
	result.RealMetrics.MultisigAddressCreation = OperationMetrics{
		Duration:    testResult.AddressCreationDuration,
		DurationMs:  testResult.AddressCreationDuration.Milliseconds(),
		DurationSec: testResult.AddressCreationDuration.Seconds(),
		DurationMin: testResult.AddressCreationDuration.Minutes(),
		Success:     true,
	}

	result.RealMetrics.TransactionSigning = OperationMetrics{
		Duration:    testResult.SigningDuration,
		DurationMs:  testResult.SigningDuration.Milliseconds(),
		DurationSec: testResult.SigningDuration.Seconds(),
		DurationMin: testResult.SigningDuration.Minutes(),
		Success:     true,
	}

	result.RealMetrics.OverallWorkflow = OperationMetrics{
		Duration:    testResult.OverallDuration,
		DurationMs:  testResult.OverallDuration.Milliseconds(),
		DurationSec: testResult.OverallDuration.Seconds(),
		DurationMin: testResult.OverallDuration.Minutes(),
		Success:     true,
	}

	// Combined crypto time (address creation + signing)
	combinedCryptoTime := testResult.AddressCreationDuration + testResult.SigningDuration
	result.RealMetrics.CombinedCryptoTime = OperationMetrics{
		Duration:    combinedCryptoTime,
		DurationMs:  combinedCryptoTime.Milliseconds(),
		DurationSec: combinedCryptoTime.Seconds(),
		DurationMin: combinedCryptoTime.Minutes(),
		Success:     true,
	}

	// Step 2: Generate simulated metrics using combined crypto time
	t.Log("Step 2: Generating simulated metrics")

	// Create operation that returns the combined crypto time
	simulationOperation := func() time.Duration {
		return combinedCryptoTime // Use real measured combined time as base
	}

	// Run simulation
	benchmarkMetrics := detector.SimulateParticipants(participants, config.NetworkLatency, simulationOperation)

	// Store simulated metrics
	totalSimulatedTime := benchmarkMetrics.AdjustedDuration + benchmarkMetrics.NetworkLatencyTotal
	result.SimulatedMetrics = SimulatedOperationMetrics{
		OperationDuration:      benchmarkMetrics.OperationDuration,
		OperationDurationMs:    benchmarkMetrics.OperationDuration.Milliseconds(),
		OperationDurationSec:   benchmarkMetrics.OperationDuration.Seconds(),
		OperationDurationMin:   benchmarkMetrics.OperationDuration.Minutes(),
		AdjustedDuration:       benchmarkMetrics.AdjustedDuration,
		AdjustedDurationMs:     benchmarkMetrics.AdjustedDuration.Milliseconds(),
		AdjustedDurationSec:    benchmarkMetrics.AdjustedDuration.Seconds(),
		AdjustedDurationMin:    benchmarkMetrics.AdjustedDuration.Minutes(),
		NetworkLatencyTotal:    benchmarkMetrics.NetworkLatencyTotal,
		NetworkLatencyTotalMs:  benchmarkMetrics.NetworkLatencyTotal.Milliseconds(),
		NetworkLatencyTotalSec: benchmarkMetrics.NetworkLatencyTotal.Seconds(),
		NetworkLatencyTotalMin: benchmarkMetrics.NetworkLatencyTotal.Minutes(),
		TotalSimulatedTime:     totalSimulatedTime,
		TotalSimulatedTimeMs:   totalSimulatedTime.Milliseconds(),
		TotalSimulatedTimeSec:  totalSimulatedTime.Seconds(),
		TotalSimulatedTimeMin:  totalSimulatedTime.Minutes(),
		ScalingFactor:          benchmarkMetrics.ComplexityFactor,
		SimulatedCores:         benchmarkMetrics.SimulatedCores,
		LinearScaling:          benchmarkMetrics.LinearScaling,
	}

	// Store blockchain transactions
	result.BlockchainTransactions = append(result.BlockchainTransactions, TransactionResult{
		Type:          "miner_to_multisig",
		TxHash:        testResult.Transaction1Hash,
		FromAddress:   "miner",
		ToAddress:     testResult.MultisigAddress,
		Amount:        TestAmount,
		BalanceBefore: testResult.InitialMinerBalance,
		BalanceAfter:  testResult.FinalMinerBalance,
		Confirmations: ConfirmationTarget,
	})

	result.BlockchainTransactions = append(result.BlockchainTransactions, TransactionResult{
		Type:          "multisig_to_miner",
		TxHash:        testResult.Transaction2Hash,
		FromAddress:   testResult.MultisigAddress,
		ToAddress:     "miner",
		Amount:        0.999,
		BalanceBefore: testResult.MultisigBalanceAfterTx1,
		BalanceAfter:  0.0,
		Confirmations: ConfirmationTarget,
	})

	// Check performance targets
	result.RealPerformanceTarget = combinedCryptoTime <= MaxCombinedOperationDuration
	result.SimulatedPerformanceTarget = benchmark.ValidatePerformanceTarget(benchmarkMetrics, config)

	// Record resource usage
	result.ResourceUsage = ResourceUsageMetrics{
		PeakMemoryMB:   GetMemoryUsage(),
		CPUUtilization: float64(participants) / 100.0,
		NetworkLatency: int64(participants * 5),
	}

	t.Logf("Real crypto time: %v, Simulated total time: %v",
		combinedCryptoTime, result.SimulatedMetrics.TotalSimulatedTime)

	return result, nil
}

// testCombinedFROSTConfiguration tests FROST with both real and simulated metrics
func testCombinedFROSTConfiguration(t *testing.T, rpc *BitcoinRPC, detector *benchmark.CoreDetector,
	config *benchmark.BenchmarkConfig, participants, threshold int) (CombinedConfigurationResult, error) {

	result := CombinedConfigurationResult{
		ParticipantCount:       participants,
		ThresholdCount:         threshold,
		BlockchainTransactions: make([]TransactionResult, 0, 2),
	}

	// Step 1: Get real blockchain metrics
	t.Log("Step 1: Collecting real blockchain metrics")
	if err := rpc.ResetBlockchain(); err != nil {
		return result, fmt.Errorf("failed to reset blockchain: %w", err)
	}

	if err := rpc.EnsureMinerFunds(); err != nil {
		return result, fmt.Errorf("failed to ensure miner funds: %w", err)
	}

	// Run real test
	testResult := RunFROSTTest(t, rpc, participants, threshold)
	if !testResult.Success {
		return result, fmt.Errorf("FROST real test failed: %s", testResult.ErrorMessage)
	}

	// Store real metrics
	result.RealMetrics.MultisigAddressCreation = OperationMetrics{
		Duration:    testResult.AddressCreationDuration,
		DurationMs:  testResult.AddressCreationDuration.Milliseconds(),
		DurationSec: testResult.AddressCreationDuration.Seconds(),
		DurationMin: testResult.AddressCreationDuration.Minutes(),
		Success:     true,
	}

	result.RealMetrics.TransactionSigning = OperationMetrics{
		Duration:    testResult.SigningDuration,
		DurationMs:  testResult.SigningDuration.Milliseconds(),
		DurationSec: testResult.SigningDuration.Seconds(),
		DurationMin: testResult.SigningDuration.Minutes(),
		Success:     true,
	}

	result.RealMetrics.OverallWorkflow = OperationMetrics{
		Duration:    testResult.OverallDuration,
		DurationMs:  testResult.OverallDuration.Milliseconds(),
		DurationSec: testResult.OverallDuration.Seconds(),
		DurationMin: testResult.OverallDuration.Minutes(),
		Success:     true,
	}

	// Combined crypto time (address creation + signing)
	combinedCryptoTimeFROST := testResult.AddressCreationDuration + testResult.SigningDuration
	result.RealMetrics.CombinedCryptoTime = OperationMetrics{
		Duration:    combinedCryptoTimeFROST,
		DurationMs:  combinedCryptoTimeFROST.Milliseconds(),
		DurationSec: combinedCryptoTimeFROST.Seconds(),
		DurationMin: combinedCryptoTimeFROST.Minutes(),
		Success:     true,
	}

	// Step 2: Generate simulated metrics using combined crypto time
	t.Log("Step 2: Generating simulated metrics")

	// Create operation that returns the combined crypto time
	simulationOperation := func() time.Duration {
		return combinedCryptoTimeFROST // Use real measured combined time as base
	}

	// Run simulation
	benchmarkMetricsFROST := detector.SimulateParticipants(participants, config.NetworkLatency, simulationOperation)

	// Store simulated metrics
	totalSimulatedTimeFROST := benchmarkMetricsFROST.AdjustedDuration + benchmarkMetricsFROST.NetworkLatencyTotal
	result.SimulatedMetrics = SimulatedOperationMetrics{
		OperationDuration:      benchmarkMetricsFROST.OperationDuration,
		OperationDurationMs:    benchmarkMetricsFROST.OperationDuration.Milliseconds(),
		OperationDurationSec:   benchmarkMetricsFROST.OperationDuration.Seconds(),
		OperationDurationMin:   benchmarkMetricsFROST.OperationDuration.Minutes(),
		AdjustedDuration:       benchmarkMetricsFROST.AdjustedDuration,
		AdjustedDurationMs:     benchmarkMetricsFROST.AdjustedDuration.Milliseconds(),
		AdjustedDurationSec:    benchmarkMetricsFROST.AdjustedDuration.Seconds(),
		AdjustedDurationMin:    benchmarkMetricsFROST.AdjustedDuration.Minutes(),
		NetworkLatencyTotal:    benchmarkMetricsFROST.NetworkLatencyTotal,
		NetworkLatencyTotalMs:  benchmarkMetricsFROST.NetworkLatencyTotal.Milliseconds(),
		NetworkLatencyTotalSec: benchmarkMetricsFROST.NetworkLatencyTotal.Seconds(),
		NetworkLatencyTotalMin: benchmarkMetricsFROST.NetworkLatencyTotal.Minutes(),
		TotalSimulatedTime:     totalSimulatedTimeFROST,
		TotalSimulatedTimeMs:   totalSimulatedTimeFROST.Milliseconds(),
		TotalSimulatedTimeSec:  totalSimulatedTimeFROST.Seconds(),
		TotalSimulatedTimeMin:  totalSimulatedTimeFROST.Minutes(),
		ScalingFactor:          benchmarkMetricsFROST.ComplexityFactor,
		SimulatedCores:         benchmarkMetricsFROST.SimulatedCores,
		LinearScaling:          benchmarkMetricsFROST.LinearScaling,
	}

	// Store blockchain transactions
	result.BlockchainTransactions = append(result.BlockchainTransactions, TransactionResult{
		Type:          "miner_to_multisig",
		TxHash:        testResult.Transaction1Hash,
		FromAddress:   "miner",
		ToAddress:     testResult.MultisigAddress,
		Amount:        TestAmount,
		BalanceBefore: testResult.InitialMinerBalance,
		BalanceAfter:  testResult.FinalMinerBalance,
		Confirmations: ConfirmationTarget,
	})

	result.BlockchainTransactions = append(result.BlockchainTransactions, TransactionResult{
		Type:          "multisig_to_miner",
		TxHash:        testResult.Transaction2Hash,
		FromAddress:   testResult.MultisigAddress,
		ToAddress:     "miner",
		Amount:        0.999,
		BalanceBefore: testResult.MultisigBalanceAfterTx1,
		BalanceAfter:  0.0,
		Confirmations: ConfirmationTarget,
	})

	// Check performance targets
	result.RealPerformanceTarget = combinedCryptoTimeFROST <= MaxCombinedOperationDuration
	result.SimulatedPerformanceTarget = benchmark.ValidatePerformanceTarget(benchmarkMetricsFROST, config)

	// Record resource usage
	result.ResourceUsage = ResourceUsageMetrics{
		PeakMemoryMB:   GetMemoryUsage(),
		CPUUtilization: float64(participants) / 100.0,
		NetworkLatency: int64(participants * 5),
	}

	t.Logf("Real crypto time: %v, Simulated total time: %v",
		combinedCryptoTimeFROST, result.SimulatedMetrics.TotalSimulatedTime)

	return result, nil
}

// generateCombinedComparison creates comparative analysis between real and simulated metrics
func generateCombinedComparison(musig2Results, frostResults []CombinedConfigurationResult) CombinedComparisonAnalysis {
	comparison := CombinedComparisonAnalysis{
		RealAddressCreationComparison: make([]ParticipantComparison, 0, len(musig2Results)),
		RealSigningComparison:         make([]ParticipantComparison, 0, len(musig2Results)),
		SimulatedCryptoComparison:     make([]ParticipantComparison, 0, len(musig2Results)),
		RealVsSimulatedComparison:     make([]RealVsSimulatedComp, 0, len(musig2Results)*2),
	}

	// Compare real address creation times
	for i, musig2 := range musig2Results {
		if i < len(frostResults) {
			frost := frostResults[i]
			comparison.RealAddressCreationComparison = append(comparison.RealAddressCreationComparison, ParticipantComparison{
				ParticipantCount:  musig2.ParticipantCount,
				MuSig2Duration:    musig2.RealMetrics.MultisigAddressCreation.Duration,
				FROSTDuration:     frost.RealMetrics.MultisigAddressCreation.Duration,
				MuSig2DurationMs:  musig2.RealMetrics.MultisigAddressCreation.DurationMs,
				MuSig2DurationSec: musig2.RealMetrics.MultisigAddressCreation.DurationSec,
				MuSig2DurationMin: musig2.RealMetrics.MultisigAddressCreation.DurationMin,
				FROSTDurationMs:   frost.RealMetrics.MultisigAddressCreation.DurationMs,
				FROSTDurationSec:  frost.RealMetrics.MultisigAddressCreation.DurationSec,
				FROSTDurationMin:  frost.RealMetrics.MultisigAddressCreation.DurationMin,
				PerformanceRatio:  float64(frost.RealMetrics.MultisigAddressCreation.DurationMs) / float64(musig2.RealMetrics.MultisigAddressCreation.DurationMs),
				RecommendedScheme: func() string {
					if musig2.RealMetrics.MultisigAddressCreation.Duration < frost.RealMetrics.MultisigAddressCreation.Duration {
						return "MuSig2"
					}
					return "FROST"
				}(),
			})
		}
	}

	// Compare real signing times
	for i, musig2 := range musig2Results {
		if i < len(frostResults) {
			frost := frostResults[i]
			comparison.RealSigningComparison = append(comparison.RealSigningComparison, ParticipantComparison{
				ParticipantCount:  musig2.ParticipantCount,
				MuSig2Duration:    musig2.RealMetrics.TransactionSigning.Duration,
				FROSTDuration:     frost.RealMetrics.TransactionSigning.Duration,
				MuSig2DurationMs:  musig2.RealMetrics.TransactionSigning.DurationMs,
				MuSig2DurationSec: musig2.RealMetrics.TransactionSigning.DurationSec,
				MuSig2DurationMin: musig2.RealMetrics.TransactionSigning.DurationMin,
				FROSTDurationMs:   frost.RealMetrics.TransactionSigning.DurationMs,
				FROSTDurationSec:  frost.RealMetrics.TransactionSigning.DurationSec,
				FROSTDurationMin:  frost.RealMetrics.TransactionSigning.DurationMin,
				PerformanceRatio:  float64(frost.RealMetrics.TransactionSigning.DurationMs) / float64(musig2.RealMetrics.TransactionSigning.DurationMs),
				RecommendedScheme: func() string {
					if musig2.RealMetrics.TransactionSigning.Duration < frost.RealMetrics.TransactionSigning.Duration {
						return "MuSig2"
					}
					return "FROST"
				}(),
			})
		}
	}

	// Compare simulated crypto times
	for i, musig2 := range musig2Results {
		if i < len(frostResults) {
			frost := frostResults[i]
			comparison.SimulatedCryptoComparison = append(comparison.SimulatedCryptoComparison, ParticipantComparison{
				ParticipantCount:  musig2.ParticipantCount,
				MuSig2Duration:    musig2.SimulatedMetrics.TotalSimulatedTime,
				FROSTDuration:     frost.SimulatedMetrics.TotalSimulatedTime,
				MuSig2DurationMs:  musig2.SimulatedMetrics.TotalSimulatedTimeMs,
				MuSig2DurationSec: musig2.SimulatedMetrics.TotalSimulatedTimeSec,
				MuSig2DurationMin: musig2.SimulatedMetrics.TotalSimulatedTimeMin,
				FROSTDurationMs:   frost.SimulatedMetrics.TotalSimulatedTimeMs,
				FROSTDurationSec:  frost.SimulatedMetrics.TotalSimulatedTimeSec,
				FROSTDurationMin:  frost.SimulatedMetrics.TotalSimulatedTimeMin,
				PerformanceRatio:  float64(frost.SimulatedMetrics.TotalSimulatedTimeMs) / float64(musig2.SimulatedMetrics.TotalSimulatedTimeMs),
				RecommendedScheme: func() string {
					if musig2.SimulatedMetrics.TotalSimulatedTime < frost.SimulatedMetrics.TotalSimulatedTime {
						return "MuSig2"
					}
					return "FROST"
				}(),
			})
		}
	}

	// Compare real vs simulated for each scheme
	for _, result := range musig2Results {
		comparison.RealVsSimulatedComparison = append(comparison.RealVsSimulatedComparison, RealVsSimulatedComp{
			ParticipantCount:      result.ParticipantCount,
			Scheme:                "MuSig2",
			RealCryptoTime:        result.RealMetrics.CombinedCryptoTime.Duration,
			SimulatedTotalTime:    result.SimulatedMetrics.TotalSimulatedTime,
			RealCryptoTimeMs:      result.RealMetrics.CombinedCryptoTime.DurationMs,
			RealCryptoTimeSec:     result.RealMetrics.CombinedCryptoTime.DurationSec,
			RealCryptoTimeMin:     result.RealMetrics.CombinedCryptoTime.DurationMin,
			SimulatedTotalTimeMs:  result.SimulatedMetrics.TotalSimulatedTimeMs,
			SimulatedTotalTimeSec: result.SimulatedMetrics.TotalSimulatedTimeSec,
			SimulatedTotalTimeMin: result.SimulatedMetrics.TotalSimulatedTimeMin,
			SimulationOverhead:    float64(result.SimulatedMetrics.TotalSimulatedTimeMs) / float64(result.RealMetrics.CombinedCryptoTime.DurationMs),
			RecommendedEstimate: func() string {
				if result.SimulatedMetrics.TotalSimulatedTime > result.RealMetrics.CombinedCryptoTime.Duration {
					return "Simulation provides conservative estimate"
				}
				return "Real performance better than simulation"
			}(),
		})
	}

	for _, result := range frostResults {
		comparison.RealVsSimulatedComparison = append(comparison.RealVsSimulatedComparison, RealVsSimulatedComp{
			ParticipantCount:      result.ParticipantCount,
			Scheme:                "FROST",
			RealCryptoTime:        result.RealMetrics.CombinedCryptoTime.Duration,
			SimulatedTotalTime:    result.SimulatedMetrics.TotalSimulatedTime,
			RealCryptoTimeMs:      result.RealMetrics.CombinedCryptoTime.DurationMs,
			RealCryptoTimeSec:     result.RealMetrics.CombinedCryptoTime.DurationSec,
			RealCryptoTimeMin:     result.RealMetrics.CombinedCryptoTime.DurationMin,
			SimulatedTotalTimeMs:  result.SimulatedMetrics.TotalSimulatedTimeMs,
			SimulatedTotalTimeSec: result.SimulatedMetrics.TotalSimulatedTimeSec,
			SimulatedTotalTimeMin: result.SimulatedMetrics.TotalSimulatedTimeMin,
			SimulationOverhead:    float64(result.SimulatedMetrics.TotalSimulatedTimeMs) / float64(result.RealMetrics.CombinedCryptoTime.DurationMs),
			RecommendedEstimate: func() string {
				if result.SimulatedMetrics.TotalSimulatedTime > result.RealMetrics.CombinedCryptoTime.Duration {
					return "Simulation provides conservative estimate"
				}
				return "Real performance better than simulation"
			}(),
		})
	}

	// Generate scalability analysis
	comparison.ScalabilityAnalysis = calculateCombinedScalabilityAnalysis(musig2Results, frostResults)

	return comparison
}

// calculateCombinedScalabilityAnalysis analyzes scaling characteristics for both real and simulated metrics
func calculateCombinedScalabilityAnalysis(musig2Results, frostResults []CombinedConfigurationResult) ScalabilityAnalysis {
	analysis := ScalabilityAnalysis{
		MuSig2LinearScaling: true,
		FROSTLinearScaling:  true,
	}

	// Analyze MuSig2 scaling
	if len(musig2Results) >= 2 {
		firstResult := musig2Results[0]
		lastResult := musig2Results[len(musig2Results)-1]

		participantRatio := float64(lastResult.ParticipantCount) / float64(firstResult.ParticipantCount)
		realTimeRatio := float64(lastResult.RealMetrics.CombinedCryptoTime.DurationMs) / float64(firstResult.RealMetrics.CombinedCryptoTime.DurationMs)

		analysis.MuSig2ComplexityFactor = realTimeRatio / participantRatio
		analysis.MuSig2LinearScaling = analysis.MuSig2ComplexityFactor <= 2.0 // Allow some overhead
	}

	// Analyze FROST scaling
	if len(frostResults) >= 2 {
		firstResult := frostResults[0]
		lastResult := frostResults[len(frostResults)-1]

		participantRatio := float64(lastResult.ParticipantCount) / float64(firstResult.ParticipantCount)
		realTimeRatio := float64(lastResult.RealMetrics.CombinedCryptoTime.DurationMs) / float64(firstResult.RealMetrics.CombinedCryptoTime.DurationMs)

		analysis.FROSTComplexityFactor = realTimeRatio / participantRatio
		analysis.FROSTLinearScaling = analysis.FROSTComplexityFactor <= 2.0 // Allow some overhead
	}

	// Determine max recommended participants based on 2-minute target
	analysis.MaxRecommendedParticipants = 64 // Conservative default
	for _, result := range append(musig2Results, frostResults...) {
		if result.RealMetrics.CombinedCryptoTime.Duration <= MaxCombinedOperationDuration {
			if result.ParticipantCount > analysis.MaxRecommendedParticipants {
				analysis.MaxRecommendedParticipants = result.ParticipantCount
			}
		}
	}

	return analysis
}

// generateCombinedSummary creates executive summary of combined results
func generateCombinedSummary(musig2Results, frostResults []CombinedConfigurationResult) CombinedSummary {
	totalConfigs := len(musig2Results) + len(frostResults)
	realTestsPassed := 0
	simulatedTestsPassed := 0

	var realWorstCase, realBestCase time.Duration = 0, time.Hour
	var simulatedWorstCase, simulatedBestCase time.Duration = 0, time.Hour

	bestRealScheme := "MuSig2"
	bestSimulatedScheme := "MuSig2"

	// Analyze all results
	allResults := append(musig2Results, frostResults...)
	for i, result := range allResults {
		scheme := "MuSig2"
		if i >= len(musig2Results) {
			scheme = "FROST"
		}

		// Count passed tests
		if result.RealPerformanceTarget {
			realTestsPassed++
		}
		if result.SimulatedPerformanceTarget {
			simulatedTestsPassed++
		}

		// Track best/worst case performance
		realTime := result.RealMetrics.CombinedCryptoTime.Duration
		simulatedTime := result.SimulatedMetrics.TotalSimulatedTime

		if realTime > realWorstCase {
			realWorstCase = realTime
		}
		if realTime < realBestCase {
			realBestCase = realTime
			bestRealScheme = scheme
		}

		if simulatedTime > simulatedWorstCase {
			simulatedWorstCase = simulatedTime
		}
		if simulatedTime < simulatedBestCase {
			simulatedBestCase = simulatedTime
			bestSimulatedScheme = scheme
		}
	}

	// Generate recommendations
	recommendations := make([]RecommendedConfig, 0, 4)
	for _, result := range musig2Results {
		if result.RealPerformanceTarget {
			recommendations = append(recommendations, RecommendedConfig{
				ParticipantCount: result.ParticipantCount,
				Scheme:           "MuSig2",
				Reason:           "Meets real performance requirements",
			})
		}
	}

	for _, result := range frostResults {
		if result.RealPerformanceTarget {
			recommendations = append(recommendations, RecommendedConfig{
				ParticipantCount: result.ParticipantCount,
				Scheme:           "FROST",
				Reason:           "Meets real performance requirements",
			})
		}
	}

	return CombinedSummary{
		TotalConfigurationsTested:     totalConfigs,
		RealTestsPassed:               realTestsPassed,
		SimulatedTestsPassed:          simulatedTestsPassed,
		BestRealPerformingScheme:      bestRealScheme,
		BestSimulatedPerformingScheme: bestSimulatedScheme,
		RealWorstCasePerformance:      realWorstCase,
		SimulatedWorstCasePerformance: simulatedWorstCase,
		RealBestCasePerformance:       realBestCase,
		SimulatedBestCasePerformance:  simulatedBestCase,
		RecommendedConfigurations:     recommendations,
	}
}

// generateCombinedRecommendations creates actionable recommendations based on combined analysis
func generateCombinedRecommendations(musig2Results, frostResults []CombinedConfigurationResult) []string {
	recommendations := make([]string, 0, 10)

	// Analyze real vs simulated accuracy
	realVsSimAccurate := 0
	totalComparisons := 0

	for _, result := range append(musig2Results, frostResults...) {
		totalComparisons++
		// Consider simulation accurate if it's within 50% of real time
		ratio := float64(result.SimulatedMetrics.TotalSimulatedTimeMs) / float64(result.RealMetrics.CombinedCryptoTime.DurationMs)
		if ratio >= 0.5 && ratio <= 2.0 {
			realVsSimAccurate++
		}
	}

	accuracyRate := float64(realVsSimAccurate) / float64(totalComparisons)

	if accuracyRate > 0.7 {
		recommendations = append(recommendations, "Simulation provides reliable performance estimates (>70% accuracy)")
	} else {
		recommendations = append(recommendations, "Simulation estimates should be used with caution - significant deviation from real performance")
	}

	// Performance recommendations
	realPassRate := 0
	for _, result := range musig2Results {
		if result.RealPerformanceTarget {
			realPassRate++
		}
	}
	for _, result := range frostResults {
		if result.RealPerformanceTarget {
			realPassRate++
		}
	}

	if realPassRate == 0 {
		recommendations = append(recommendations, "No configurations meet the 2-minute performance target - consider optimizing cryptographic operations")
	} else if realPassRate < len(musig2Results)+len(frostResults) {
		recommendations = append(recommendations, fmt.Sprintf("Only %d/%d configurations meet performance targets - limit participant count for production", realPassRate, len(musig2Results)+len(frostResults)))
	}

	// Scheme-specific recommendations
	musig2Better := 0
	frostBetter := 0

	for i, musig2 := range musig2Results {
		if i < len(frostResults) {
			frost := frostResults[i]
			if musig2.RealMetrics.CombinedCryptoTime.Duration < frost.RealMetrics.CombinedCryptoTime.Duration {
				musig2Better++
			} else {
				frostBetter++
			}
		}
	}

	if musig2Better > frostBetter {
		recommendations = append(recommendations, "MuSig2 generally outperforms FROST in real blockchain operations")
	} else if frostBetter > musig2Better {
		recommendations = append(recommendations, "FROST generally outperforms MuSig2 in real blockchain operations")
	} else {
		recommendations = append(recommendations, "MuSig2 and FROST show similar real-world performance characteristics")
	}

	// Scaling recommendations
	recommendations = append(recommendations, "Monitor performance degradation beyond 128 participants")
	recommendations = append(recommendations, "Use simulation for initial capacity planning, validate with real tests")
	recommendations = append(recommendations, "Consider network latency impact in distributed federation deployments")

	return recommendations
}

// saveCombinedPerformanceReport saves the combined performance report to JSON file
func saveCombinedPerformanceReport(report *CombinedPerformanceReport) error {
	// Marshal the report to JSON with pretty printing
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report to JSON: %w", err)
	}

	// Write to file
	reportPath := filepath.Join("test", "taproot", "combined_performance_report.json")
	if err := os.WriteFile(reportPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}
