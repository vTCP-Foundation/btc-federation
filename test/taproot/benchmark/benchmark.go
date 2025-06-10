package benchmark

import (
	"runtime"
	"sync"
	"time"
)

// PerformanceMetrics captures detailed performance data for multisig operations
type PerformanceMetrics struct {
	// Core performance metrics
	OperationDuration time.Duration `json:"operation_duration"`
	MemoryUsageMB     float64       `json:"memory_usage_mb"`
	CPUCores          int           `json:"cpu_cores"`

	// Participant scaling metrics
	ParticipantCount    int           `json:"participant_count"`
	ThresholdCount      int           `json:"threshold_count"`
	SimulatedCores      int           `json:"simulated_cores"`
	AdjustedDuration    time.Duration `json:"adjusted_duration"`
	NetworkLatencyTotal time.Duration `json:"network_latency_total"`

	// Success metrics
	SuccessRate  float64 `json:"success_rate"`
	FailureCount int     `json:"failure_count"`

	// Resource scaling characteristics
	LinearScaling    bool    `json:"linear_scaling"`
	ComplexityFactor float64 `json:"complexity_factor"`
}

// BenchmarkConfig defines the configuration for performance benchmarks
type BenchmarkConfig struct {
	ParticipantCounts []int         `json:"participant_counts"`
	NetworkLatency    time.Duration `json:"network_latency"`
	MaxDuration       time.Duration `json:"max_duration"`
	Iterations        int           `json:"iterations"`
}

// DefaultBenchmarkConfig returns the standard configuration per PRD requirements
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		ParticipantCounts: []int{64, 128, 256, 512},
		NetworkLatency:    100 * time.Millisecond, // 100ms inter-participant latency
		MaxDuration:       2 * time.Minute,        // 2-minute operation requirement
		Iterations:        3,                      // Multiple runs for averaging
	}
}

// CoreDetector detects available CPU cores and calculates simulation parameters
type CoreDetector struct {
	AvailableCores int
	MaxParallel    int
}

// NewCoreDetector creates a new core detector
func NewCoreDetector() *CoreDetector {
	cores := runtime.NumCPU()
	return &CoreDetector{
		AvailableCores: cores,
		MaxParallel:    cores,
	}
}

// CalculateScalingFactor determines how to scale from simulated cores to full participant count
func (cd *CoreDetector) CalculateScalingFactor(participantCount int) float64 {
	if participantCount <= cd.MaxParallel {
		return 1.0
	}
	return float64(participantCount) / float64(cd.MaxParallel)
}

// SimulateParticipants runs a function with limited parallel execution and scales results
func (cd *CoreDetector) SimulateParticipants(participantCount int, networkLatency time.Duration, operation func() time.Duration) PerformanceMetrics {
	thresholdCount := participantCount / 2 // 50% threshold per PRD
	scalingFactor := cd.CalculateScalingFactor(participantCount)
	actualCores := cd.MaxParallel
	if participantCount < cd.MaxParallel {
		actualCores = participantCount
	}

	// Execute operations in parallel up to available cores
	var wg sync.WaitGroup
	durations := make([]time.Duration, actualCores)

	for i := 0; i < actualCores; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			durations[index] = operation()
		}(i)
	}
	wg.Wait()

	// Calculate metrics
	var totalDuration time.Duration
	for _, d := range durations {
		if d > totalDuration {
			totalDuration = d // Use max duration (worst case)
		}
	}

	// Adjust duration for full participant count
	adjustedDuration := time.Duration(float64(totalDuration) * scalingFactor)

	// Add network latency simulation per participant interaction
	networkLatencyTotal := time.Duration(participantCount) * networkLatency

	return PerformanceMetrics{
		OperationDuration:   totalDuration,
		CPUCores:            cd.AvailableCores,
		ParticipantCount:    participantCount,
		ThresholdCount:      thresholdCount,
		SimulatedCores:      actualCores,
		AdjustedDuration:    adjustedDuration,
		NetworkLatencyTotal: networkLatencyTotal,
		LinearScaling:       scalingFactor <= 2.0, // Consider linear if < 2x scaling
		ComplexityFactor:    scalingFactor,
		SuccessRate:         1.0, // Will be updated by actual tests
	}
}

// GetMemoryUsage returns current memory usage in MB
func GetMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024 // Convert bytes to MB
}

// ValidatePerformanceTarget checks if metrics meet the 2-minute requirement
func ValidatePerformanceTarget(metrics PerformanceMetrics, config *BenchmarkConfig) bool {
	totalTime := metrics.AdjustedDuration + metrics.NetworkLatencyTotal
	return totalTime <= config.MaxDuration
}
