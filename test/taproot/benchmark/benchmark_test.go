package benchmark

import (
	"fmt"
	"testing"
	"time"
)

func TestCoreDetection(t *testing.T) {
	detector := NewCoreDetector()

	if detector.AvailableCores <= 0 {
		t.Errorf("Expected positive core count, got %d", detector.AvailableCores)
	}

	t.Logf("Detected %d CPU cores", detector.AvailableCores)
}

func TestScalingFactor(t *testing.T) {
	detector := NewCoreDetector()

	testCases := []struct {
		participants int
		expectLinear bool
	}{
		{64, true},
		{128, detector.AvailableCores >= 128},
		{256, detector.AvailableCores >= 256},
		{512, detector.AvailableCores >= 512},
	}

	for _, tc := range testCases {
		factor := detector.CalculateScalingFactor(tc.participants)
		t.Logf("Participants: %d, Scaling factor: %.2f", tc.participants, factor)

		if tc.expectLinear && factor > 2.0 {
			t.Logf("Warning: High scaling factor %.2f for %d participants", factor, tc.participants)
		}
	}
}

func TestSimulateParticipants(t *testing.T) {
	detector := NewCoreDetector()

	// Simulate a crypto operation that takes 10ms
	mockOperation := func() time.Duration {
		time.Sleep(10 * time.Millisecond)
		return 10 * time.Millisecond
	}

	participantCounts := []int{64, 128, 256, 512}

	for _, count := range participantCounts {
		t.Run(fmt.Sprintf("Participants_%d", count), func(t *testing.T) {
			metrics := detector.SimulateParticipants(count, 200*time.Millisecond, mockOperation)

			t.Logf("Participants: %d", metrics.ParticipantCount)
			t.Logf("Threshold: %d", metrics.ThresholdCount)
			t.Logf("Simulated cores: %d", metrics.SimulatedCores)
			t.Logf("Operation duration: %v", metrics.OperationDuration)
			t.Logf("Adjusted duration: %v", metrics.AdjustedDuration)
			t.Logf("Network latency: %v", metrics.NetworkLatencyTotal)
			t.Logf("Linear scaling: %v", metrics.LinearScaling)
			t.Logf("Complexity factor: %.2f", metrics.ComplexityFactor)

			// Verify threshold is 50%
			expectedThreshold := count / 2
			if metrics.ThresholdCount != expectedThreshold {
				t.Errorf("Expected threshold %d, got %d", expectedThreshold, metrics.ThresholdCount)
			}

			// Verify network latency calculation
			expectedLatency := time.Duration(count) * 200 * time.Millisecond
			if metrics.NetworkLatencyTotal != expectedLatency {
				t.Errorf("Expected network latency %v, got %v", expectedLatency, metrics.NetworkLatencyTotal)
			}
		})
	}
}

func TestPerformanceValidation(t *testing.T) {
	config := DefaultBenchmarkConfig()

	testCases := []struct {
		name             string
		adjustedDuration time.Duration
		networkLatency   time.Duration
		shouldPass       bool
	}{
		{
			name:             "Fast operation",
			adjustedDuration: 30 * time.Second,
			networkLatency:   60 * time.Second,
			shouldPass:       true, // Total: 90s < 120s
		},
		{
			name:             "Slow operation",
			adjustedDuration: 90 * time.Second,
			networkLatency:   60 * time.Second,
			shouldPass:       false, // Total: 150s > 120s
		},
		{
			name:             "Edge case",
			adjustedDuration: 60 * time.Second,
			networkLatency:   60 * time.Second,
			shouldPass:       true, // Total: 120s = 120s
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := PerformanceMetrics{
				AdjustedDuration:    tc.adjustedDuration,
				NetworkLatencyTotal: tc.networkLatency,
			}

			result := ValidatePerformanceTarget(metrics, config)
			if result != tc.shouldPass {
				t.Errorf("Expected validation %v, got %v", tc.shouldPass, result)
			}
		})
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		usage := GetMemoryUsage()
		if usage <= 0 {
			b.Errorf("Invalid memory usage: %.2f MB", usage)
		}
	}
}
