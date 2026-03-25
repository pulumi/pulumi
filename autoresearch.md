# Conformance Test Optimization — Autoresearch

## Objective
Optimize the nodejs conformance tests to run faster while maintaining correctness.

## Current Best Result
- **TSC variant: 319s** (vs 578s baseline) = **44.8% improvement**
- Key optimizations: tmpfs for temp dirs + cached test binary

## Experiments Summary

| # | Description | Time | vs Baseline | Status |
|---|------------|------|-------------|--------|
| 0 | Baseline (c5.4xlarge) | 578s | - | baseline |
| 1 | Cache binary build | 578s | 0% | keep |
| 2 | -parallel=4 | 746s | +29% | discard |
| 3 | -parallel=16 | 589s | +2% | discard |
| 4 | npm prefer-offline | crash | - | crash |
| 5 | node_modules hardlink cache | 698s | +21% | discard |
| 6 | **tmpfs for temp dirs** | **319s** | **-44.8%** | **keep** |
| 7 | tmpfs + parallel=32 | 721s | +25% | discard |

## What Works
1. **tmpfs (RAM disk)**: Biggest win. npm install + tsc do massive I/O
2. **Binary caching**: Small win per variant but accumulates across 3 variants

## What Doesn't Work
- Parallelism tuning: default is near-optimal
- node_modules caching: copy overhead > install savings
- npm_config_prefer_offline: incompatible with local tarballs
- Very high parallelism + tmpfs: causes OOM

## Key Bottleneck
- `l1-builtin-require-pulumi-version`: 138-143s (single longest test)
- This test likely does version checking involving network calls

## Implementation
- OTel tracing added to test framework
- Binary caching via sync.Once in language_test.go
- tmpfs can be enabled by setting TMPDIR=/mnt/test-tmpfs before running tests
