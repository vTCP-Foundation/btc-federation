package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"btc-federation/test/taproot/benchmark"
	"btc-federation/test/taproot/frost"
	"btc-federation/test/taproot/musig2"
)

// PerformanceReport aggregates all performance metrics for both schemes
type PerformanceReport struct {
	TestTimestamp      time.Time                  `json:"test_timestamp"`
	SystemInfo         SystemInfo                 `json:"system_info"`
	MuSig2Results      map[int]*SchemePerformance `json:"musig2_results"`
	FROSTResults       map[int]*SchemePerformance `json:"frost_results"`
	ComparisonAnalysis *ComparisonAnalysis        `json:"comparison_analysis"`
	Summary            *PerformanceSummary        `json:"summary"`
}

// SystemInfo captures system configuration details
type SystemInfo struct {
	CPUCores       int    `json:"cpu_cores"`
	TestDuration   string `json:"test_duration"`
	GoVersion      string `json:"go_version"`
	NetworkLatency string `json:"network_latency"`
}

// SchemePerformance contains detailed performance metrics for a specific scheme
type SchemePerformance struct {
	ParticipantCount       int                           `json:"participant_count"`
	ThresholdCount         int                           `json:"threshold_count"`
	Metrics                *benchmark.PerformanceMetrics `json:"metrics"`
	PhaseBreakdown         map[string]time.Duration      `json:"phase_breakdown"`
	TargetMet              bool                          `json:"target_met"`
	ScalingCharacteristics *ScalingCharacteristics       `json:"scaling_characteristics"`
}

// ScalingCharacteristics analyzes how performance scales with participant count
type ScalingCharacteristics struct {
	IsLinear         bool   `json:"is_linear"`
	ComplexityGrowth string `json:"complexity_growth"`
	MemoryScaling    string `json:"memory_scaling"`
	RecommendedLimit int    `json:"recommended_limit"`
}

// ComparisonAnalysis compares MuSig2 vs FROST performance
type ComparisonAnalysis struct {
	BetterScheme      map[int]string  `json:"better_scheme"`
	PerformanceGap    map[int]float64 `json:"performance_gap"`
	ScalabilityWinner string          `json:"scalability_winner"`
	Recommendations   []string        `json:"recommendations"`
}

// PerformanceSummary provides high-level summary of all results
type PerformanceSummary struct {
	MaxViableParticipants map[string]int `json:"max_viable_participants"`
	TargetMetCounts       map[string]int `json:"target_met_counts"`
	OverallRecommendation string         `json:"overall_recommendation"`
	CriticalFindings      []string       `json:"critical_findings"`
}

func TestComprehensivePerformanceAnalysis(t *testing.T) {
	detector := benchmark.NewCoreDetector()
	config := benchmark.DefaultBenchmarkConfig()
	message := sha256.Sum256([]byte("comprehensive performance test"))

	t.Logf("=== Starting Comprehensive Taproot Multisig Performance Analysis ===")
	t.Logf("System: %d CPU cores available", detector.AvailableCores)
	t.Logf("Target: All operations complete within %v", config.MaxDuration)
	t.Logf("Network latency simulation: %v per participant", config.NetworkLatency)

	report := &PerformanceReport{
		TestTimestamp: time.Now(),
		SystemInfo: SystemInfo{
			CPUCores:       detector.AvailableCores,
			TestDuration:   config.MaxDuration.String(),
			NetworkLatency: config.NetworkLatency.String(),
		},
		MuSig2Results: make(map[int]*SchemePerformance),
		FROSTResults:  make(map[int]*SchemePerformance),
	}

	// Test MuSig2 performance
	t.Run("MuSig2_Performance_Analysis", func(t *testing.T) {
		for _, participantCount := range config.ParticipantCounts {
			t.Run(fmt.Sprintf("MuSig2_%d_participants", participantCount), func(t *testing.T) {
				performance := testMuSig2Performance(t, participantCount, message[:], detector, config)
				report.MuSig2Results[participantCount] = performance

				t.Logf("MuSig2 %d participants: %v total time, target met: %v",
					participantCount,
					performance.Metrics.AdjustedDuration+performance.Metrics.NetworkLatencyTotal,
					performance.TargetMet)
			})
		}
	})

	// Test FROST performance
	t.Run("FROST_Performance_Analysis", func(t *testing.T) {
		for _, participantCount := range config.ParticipantCounts {
			t.Run(fmt.Sprintf("FROST_%d_participants", participantCount), func(t *testing.T) {
				performance := testFROSTPerformance(t, participantCount, message[:], detector, config)
				report.FROSTResults[participantCount] = performance

				t.Logf("FROST %d participants: %v total time, target met: %v",
					participantCount,
					performance.Metrics.AdjustedDuration+performance.Metrics.NetworkLatencyTotal,
					performance.TargetMet)
			})
		}
	})

	// Generate analysis
	report.ComparisonAnalysis = generateComparisonAnalysis(report)
	report.Summary = generatePerformanceSummary(report)

	// Save report
	err := savePerformanceReport(report)
	if err != nil {
		t.Errorf("Failed to save performance report: %v", err)
	} else {
		t.Logf("Performance report saved to: test/taproot/integration_report.json")
	}

	// Log critical findings
	t.Logf("\n=== PERFORMANCE ANALYSIS SUMMARY ===")
	for _, finding := range report.Summary.CriticalFindings {
		t.Logf("CRITICAL: %s", finding)
	}

	t.Logf("\nMax viable participants:")
	for scheme, maxCount := range report.Summary.MaxViableParticipants {
		t.Logf("  %s: %d participants", scheme, maxCount)
	}

	t.Logf("\nOverall recommendation: %s", report.Summary.OverallRecommendation)
}

func testMuSig2Performance(t *testing.T, participantCount int, message []byte, detector *benchmark.CoreDetector, config *benchmark.BenchmarkConfig) *SchemePerformance {
	operation := func() time.Duration {
		start := time.Now()

		session, err := musig2.NewMuSig2Session(participantCount, message)
		if err != nil {
			t.Fatalf("Failed to create MuSig2 session: %v", err)
		}

		err = session.KeyGeneration()
		if err != nil {
			t.Fatalf("MuSig2 key generation failed: %v", err)
		}

		err = session.NonceGeneration()
		if err != nil {
			t.Fatalf("MuSig2 nonce generation failed: %v", err)
		}

		err = session.PartialSign()
		if err != nil {
			t.Fatalf("MuSig2 partial signing failed: %v", err)
		}

		err = session.AggregateSignatures()
		if err != nil {
			t.Fatalf("MuSig2 signature aggregation failed: %v", err)
		}

		_, err = session.VerifySignature()
		if err != nil {
			t.Fatalf("MuSig2 signature verification failed: %v", err)
		}

		return time.Since(start)
	}

	metrics := detector.SimulateParticipants(participantCount, config.NetworkLatency, operation)
	targetMet := benchmark.ValidatePerformanceTarget(metrics, config)

	return &SchemePerformance{
		ParticipantCount:       participantCount,
		ThresholdCount:         metrics.ThresholdCount,
		Metrics:                &metrics,
		TargetMet:              targetMet,
		ScalingCharacteristics: analyzeScalingCharacteristics(metrics),
	}
}

func testFROSTPerformance(t *testing.T, participantCount int, message []byte, detector *benchmark.CoreDetector, config *benchmark.BenchmarkConfig) *SchemePerformance {
	operation := func() time.Duration {
		start := time.Now()

		session, err := frost.NewFROSTSession(participantCount, message)
		if err != nil {
			t.Fatalf("Failed to create FROST session: %v", err)
		}

		// Set the threshold count to match expected 50% threshold
		session.ThresholdCount = participantCount / 2

		err = session.DistributedKeyGeneration()
		if err != nil {
			t.Fatalf("FROST DKG failed: %v", err)
		}

		err = session.NonceGeneration()
		if err != nil {
			t.Fatalf("FROST nonce generation failed: %v", err)
		}

		err = session.PartialSign()
		if err != nil {
			t.Fatalf("FROST partial signing failed: %v", err)
		}

		err = session.AggregateSignatures()
		if err != nil {
			t.Fatalf("FROST signature aggregation failed: %v", err)
		}

		_, err = session.VerifySignature()
		if err != nil {
			t.Fatalf("FROST signature verification failed: %v", err)
		}

		return time.Since(start)
	}

	metrics := detector.SimulateParticipants(participantCount, config.NetworkLatency, operation)
	targetMet := benchmark.ValidatePerformanceTarget(metrics, config)

	return &SchemePerformance{
		ParticipantCount:       participantCount,
		ThresholdCount:         metrics.ThresholdCount,
		Metrics:                &metrics,
		TargetMet:              targetMet,
		ScalingCharacteristics: analyzeScalingCharacteristics(metrics),
	}
}

func analyzeScalingCharacteristics(metrics benchmark.PerformanceMetrics) *ScalingCharacteristics {
	var complexityGrowth string
	var memoryScaling string

	if metrics.ComplexityFactor <= 1.5 {
		complexityGrowth = "Sub-linear"
	} else if metrics.ComplexityFactor <= 2.5 {
		complexityGrowth = "Linear"
	} else if metrics.ComplexityFactor <= 4.0 {
		complexityGrowth = "Quadratic"
	} else {
		complexityGrowth = "Exponential"
	}

	// Memory scaling analysis (simplified)
	if metrics.MemoryUsageMB < 100 {
		memoryScaling = "Low"
	} else if metrics.MemoryUsageMB < 500 {
		memoryScaling = "Moderate"
	} else {
		memoryScaling = "High"
	}

	// Recommend participant limit based on complexity
	var recommendedLimit int
	if metrics.ComplexityFactor <= 2.0 {
		recommendedLimit = 512
	} else if metrics.ComplexityFactor <= 3.0 {
		recommendedLimit = 256
	} else if metrics.ComplexityFactor <= 4.0 {
		recommendedLimit = 128
	} else {
		recommendedLimit = 64
	}

	return &ScalingCharacteristics{
		IsLinear:         metrics.LinearScaling,
		ComplexityGrowth: complexityGrowth,
		MemoryScaling:    memoryScaling,
		RecommendedLimit: recommendedLimit,
	}
}

func generateComparisonAnalysis(report *PerformanceReport) *ComparisonAnalysis {
	analysis := &ComparisonAnalysis{
		BetterScheme:    make(map[int]string),
		PerformanceGap:  make(map[int]float64),
		Recommendations: []string{},
	}

	musig2Wins := 0
	frostWins := 0

	for participantCount := range report.MuSig2Results {
		musig2Perf := report.MuSig2Results[participantCount]
		frostPerf := report.FROSTResults[participantCount]

		musig2Total := musig2Perf.Metrics.AdjustedDuration + musig2Perf.Metrics.NetworkLatencyTotal
		frostTotal := frostPerf.Metrics.AdjustedDuration + frostPerf.Metrics.NetworkLatencyTotal

		if musig2Total < frostTotal {
			analysis.BetterScheme[participantCount] = "MuSig2"
			analysis.PerformanceGap[participantCount] = float64(frostTotal-musig2Total) / float64(musig2Total)
			musig2Wins++
		} else {
			analysis.BetterScheme[participantCount] = "FROST"
			analysis.PerformanceGap[participantCount] = float64(musig2Total-frostTotal) / float64(frostTotal)
			frostWins++
		}
	}

	if musig2Wins > frostWins {
		analysis.ScalabilityWinner = "MuSig2"
		analysis.Recommendations = append(analysis.Recommendations, "MuSig2 shows better overall performance across participant counts")
	} else {
		analysis.ScalabilityWinner = "FROST"
		analysis.Recommendations = append(analysis.Recommendations, "FROST shows better overall performance across participant counts")
	}

	return analysis
}

func generatePerformanceSummary(report *PerformanceReport) *PerformanceSummary {
	summary := &PerformanceSummary{
		MaxViableParticipants: make(map[string]int),
		TargetMetCounts:       make(map[string]int),
		CriticalFindings:      []string{},
	}

	// Analyze MuSig2 results
	musig2Max := 0
	musig2TargetMet := 0
	for _, perf := range report.MuSig2Results {
		if perf.TargetMet {
			if perf.ParticipantCount > musig2Max {
				musig2Max = perf.ParticipantCount
			}
			musig2TargetMet++
		}
	}
	summary.MaxViableParticipants["MuSig2"] = musig2Max
	summary.TargetMetCounts["MuSig2"] = musig2TargetMet

	// Analyze FROST results
	frostMax := 0
	frostTargetMet := 0
	for _, perf := range report.FROSTResults {
		if perf.TargetMet {
			if perf.ParticipantCount > frostMax {
				frostMax = perf.ParticipantCount
			}
			frostTargetMet++
		}
	}
	summary.MaxViableParticipants["FROST"] = frostMax
	summary.TargetMetCounts["FROST"] = frostTargetMet

	// Generate critical findings
	if musig2Max == 0 && frostMax == 0 {
		summary.CriticalFindings = append(summary.CriticalFindings, "Neither scheme meets 5-minute target for any tested participant count")
		summary.OverallRecommendation = "Re-evaluate requirements or consider alternative approaches"
	} else if musig2Max >= 512 && frostMax >= 512 {
		summary.CriticalFindings = append(summary.CriticalFindings, "Both schemes support 512+ participants within target")
		summary.OverallRecommendation = "Both schemes are viable; choose based on implementation preferences"
	} else if musig2Max >= 256 || frostMax >= 256 {
		summary.CriticalFindings = append(summary.CriticalFindings, "At least one scheme supports 256+ participants")
		if musig2Max > frostMax {
			summary.OverallRecommendation = "Recommend MuSig2 for federation sizes up to " + fmt.Sprintf("%d", musig2Max)
		} else {
			summary.OverallRecommendation = "Recommend FROST for federation sizes up to " + fmt.Sprintf("%d", frostMax)
		}
	} else {
		summary.CriticalFindings = append(summary.CriticalFindings, "Limited scalability: max participants below target federation size")
		summary.OverallRecommendation = "Consider performance optimizations or reduced federation size"
	}

	return summary
}

func savePerformanceReport(report *PerformanceReport) error {
	reportPath := filepath.Join("test", "taproot", "integration_report.json")

	// Ensure directory exists
	err := os.MkdirAll(filepath.Dir(reportPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Marshal report to JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Write to file
	err = os.WriteFile(reportPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// Threshold configuration validation tests
func TestThresholdConfigurations(t *testing.T) {
	testCases := []struct {
		participants      int
		expectedThreshold int
	}{
		{64, 32},   // 32-of-64
		{128, 64},  // 64-of-128
		{256, 128}, // 128-of-256
		{512, 256}, // 256-of-512
	}

	message := sha256.Sum256([]byte("threshold validation test"))

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Threshold_%d_of_%d", tc.expectedThreshold, tc.participants), func(t *testing.T) {
			// Test MuSig2 threshold
			t.Run("MuSig2", func(t *testing.T) {
				session, err := musig2.NewMuSig2Session(tc.participants, message[:])
				if err != nil {
					t.Fatalf("Failed to create MuSig2 session: %v", err)
				}

				if session.ThresholdCount != tc.expectedThreshold {
					t.Errorf("MuSig2 threshold mismatch: expected %d, got %d", tc.expectedThreshold, session.ThresholdCount)
				}
			})

			// Test FROST threshold
			t.Run("FROST", func(t *testing.T) {
				session, err := frost.NewFROSTSession(tc.participants, message[:])
				if err != nil {
					t.Fatalf("Failed to create FROST session: %v", err)
				}

				if session.ThresholdCount != tc.expectedThreshold {
					t.Errorf("FROST threshold mismatch: expected %d, got %d", tc.expectedThreshold, session.ThresholdCount)
				}
			})
		})
	}
}
