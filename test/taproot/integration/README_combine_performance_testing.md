# Combined Performance Metrics Test

## Overview

`TestCombinedPerformanceMetrics` is a comprehensive performance test for MuSig2 and FROST multisig schemes in Bitcoin Federation. This test implements the **01-3 Performance Metrics Collection and Report Generation** task from the PRD documentation.

## Test Objectives

The test collects and analyzes performance metrics for:
- **Real blockchain operations** (address creation and transaction signing)
- **Simulated scaling metrics** (performance forecasting for large participant counts)
- **Comparative analysis** between MuSig2 and FROST schemes

## Test Configurations

The test runs for the following configurations:
- 64 participants / 32 threshold
- 128 participants / 64 threshold  
- 256 participants / 128 threshold
- 512 participants / 256 threshold

## Test Structure

### Phase 1: MuSig2 Testing
For each configuration:
1. **Real blockchain operations**:
   - Multisig address creation
   - Transaction signing from multisig → miner
   - Measuring execution time for each operation

2. **Scaling simulation**:
   - Using measured time as baseline
   - Calculating projected time for full participant count
   - Adding network latency

### Phase 2: FROST Testing
Similar process for the FROST scheme.

### Phase 3: Analysis and Reporting
Generation of comprehensive report with comparative analysis.

## Report Format (combined_performance_report.json)

### Basic Structure
```json
{
  "generated_at": "timestamp",
  "test_environment": {...},
  "musig2_results": [...],
  "frost_results": [...],
  "comparison": {...},
  "recommendations": [...],
  "summary": {...}
}
```

### Detailed Field Description

#### 1. Test Environment
```json
"test_environment": {
  "cpu_cores": 8,
  "os": "linux",
  "go_version": "go1.21.0",
  "bitcoin_network": "regtest",
  "test_timestamp": "2024-01-01T10:00:00Z"
}
```

#### 2. Configuration Results (musig2_results / frost_results)

For each configuration:

```json
{
  "participant_count": 512,
  "threshold_count": 256,
  
  "real_metrics": {
    "multisig_address_creation": {
      "duration": "100ms",
      "duration_ms": 100,
      "duration_sec": 0.1,
      "success": true
    },
    "transaction_signing": {
      "duration": "200ms", 
      "duration_ms": 200,
      "duration_sec": 0.2,
      "success": true
    },
    "overall_workflow": {
      "duration": "350ms",
      "duration_ms": 350,
      "duration_sec": 0.35,
      "success": true
    },
    "combined_crypto_time": {
      "duration": "300ms",
      "duration_ms": 300,
      "duration_sec": 0.3,
      "success": true
    }
  },
  
  "simulated_metrics": {
    "operation_duration": "300ms",
    "operation_duration_ms": 300,
    "operation_duration_sec": 0.3,
    "adjusted_duration": "2400ms",
    "adjusted_duration_ms": 2400,
    "adjusted_duration_sec": 2.4,
    "network_latency_total": "102400ms",
    "network_latency_total_ms": 102400,
    "network_latency_total_sec": 102.4,
    "total_simulated_time": "104800ms",
    "total_simulated_time_ms": 104800,
    "total_simulated_time_sec": 104.8,
    "scaling_factor": 8.0,
    "simulated_cores": 8,
    "linear_scaling": false
  },
  
  "real_performance_target": false,
  "simulated_performance_target": true,
  
  "blockchain_transactions": [
    {
      "type": "miner_to_multisig",
      "tx_hash": "abcd1234...",
      "from_address": "miner",
      "to_address": "bcrt1p...",
      "amount": 1.0,
      "balance_before": 50.0,
      "balance_after": 49.0,
      "confirmations": 1
    }
  ],
  
  "resource_usage": {
    "peak_memory_mb": 256.5,
    "cpu_utilization": 5.12,
    "network_latency_ms": 2560
  }
}
```

#### 3. Comparison Analysis
```json
"comparison": {
  "real_address_creation_comparison": [
    {
      "participant_count": 512,
      "musig2_duration": "100ms",
      "frost_duration": "150ms",
      "musig2_duration_ms": 100,
      "musig2_duration_sec": 0.1,
      "frost_duration_ms": 150,
      "frost_duration_sec": 0.15,
      "performance_ratio": 1.5,
      "recommended_scheme": "MuSig2"
    }
  ],
  "real_signing_comparison": [...],
  "simulated_crypto_comparison": [...],
  "real_vs_simulated_comparison": [
    {
      "participant_count": 512,
      "scheme": "MuSig2",
      "real_crypto_time": "300ms",
      "simulated_total_time": "104800ms",
      "real_crypto_time_ms": 300,
      "real_crypto_time_sec": 0.3,
      "simulated_total_time_ms": 104800,
      "simulated_total_time_sec": 104.8,
      "simulation_overhead_ratio": 349.3,
      "recommended_estimate": "Simulation provides conservative estimate"
    }
  ],
  "scalability_analysis": {
    "musig2_linear_scaling": false,
    "frost_linear_scaling": false,
    "musig2_complexity_factor": 2.5,
    "frost_complexity_factor": 3.2,
    "max_recommended_participants": 256
  }
}
```

#### 4. Summary
```json
"summary": {
  "total_configurations_tested": 8,
  "real_tests_passed": 6,
  "simulated_tests_passed": 8,
  "best_real_performing_scheme": "MuSig2",
  "best_simulated_performing_scheme": "MuSig2",
  "real_worst_case_performance": "2m30s",
  "simulated_worst_case_performance": "3m45s",
  "real_best_case_performance": "45s",
  "simulated_best_case_performance": "1m20s",
  "recommended_configurations": [
    {
      "participant_count": 128,
      "scheme": "MuSig2",
      "reason": "Meets real performance requirements"
    }
  ]
}
```

## Key Metrics

### Real Metrics
- **multisig_address_creation**: Time for multisig address creation
- **transaction_signing**: Transaction signing time (includes all preparatory actions)
- **overall_workflow**: Total workflow execution time
- **combined_crypto_time**: Combined cryptographic operations time (address creation + signing)

### Simulated Metrics
- **operation_duration**: Base operation time (from real metrics)
- **adjusted_duration**: Adjusted time for full participant count
- **network_latency_total**: Simulated network latency (participantCount × NetworkLatency)
- **total_simulated_time**: Total projected time (adjusted_duration + network_latency_total)
- **scaling_factor**: Scaling coefficient
- **linear_scaling**: Whether linear scaling is maintained

### Performance Targets
- **real_performance_target**: Whether real metrics meet the 2-minute requirement
- **simulated_performance_target**: Whether simulated metrics meet the 2-minute requirement

## Important Features

### NetworkLatency Impact
The `NetworkLatency` parameter from `benchmark.DefaultBenchmarkConfig()` **affects only simulated metrics**:
- `network_latency_total = participantCount × NetworkLatency`
- Increasing NetworkLatency proportionally increases `total_simulated_time`
- **Does NOT affect** real blockchain operations

### Result Variability
- **Real metrics** may vary between runs due to:
  - System load
  - Variations in cryptographic operations
  - Network conditions
- **FROST** shows greater variability than **MuSig2**
- **Simulated metrics** are deterministic (except for base operation time)

### Recommendations
1. **For initial capacity planning** - use simulated metrics
2. **For validation** - always conduct real tests
3. **When analyzing NetworkLatency** - look at `simulated_metrics`, not `real_metrics`
4. **For production** - focus on configurations where `real_performance_target: true`

## Running the Test

```bash
go test -v ./test/taproot/integration -run TestCombinedPerformanceMetrics
```

Results will be saved to `test/taproot/combined_performance_report.json`.

## Technical Details

### Blockchain Operations
Each test executes a complete cycle:
1. Reset blockchain to genesis block
2. Ensure funds in miner wallet
3. Transaction miner → multisig (1 confirmation)
4. Transaction multisig → miner (1 confirmation)
5. Balance verification

### Scaling Simulation
- Executed on available CPU cores
- Scales to full participant count mathematically
- Adds network latency per participant
- Accounts for non-linear complexity for large groups

### Comparative Analysis
- MuSig2 vs FROST for each operation
- Real vs simulated estimates
- Scalability analysis
- Recommendations for practical federation sizes 