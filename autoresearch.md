# Conformance Test Optimization — Autoresearch

## Objective
Optimize the nodejs conformance tests (`sdk/nodejs/cmd/pulumi-language-nodejs`) to run faster while maintaining correctness.

## Benchmark Command
Run TSC conformance tests on a remote EC2 c5.4xlarge (16 vCPU, 32 GB RAM) instance.

## Primary Metric
- `tsc_ms` — Wall-clock time for TSC variant

## Current Best
- **319s** (tmpfs + cached binary) vs **578s baseline** = **44.8% improvement**

## Strategies Tried

### KEEP: Cache pulumi-test-language binary (Exp 1: 578s → saves ~15s per variant)
- Used sync.Once to build binary once across TSC/TSNode/Bun variants
- File: `sdk/nodejs/cmd/pulumi-language-nodejs/language_test.go`

### DISCARD: Reduced parallelism -parallel=4 (Exp 2: 746s — 27% SLOWER)
- Reducing parallelism significantly slows things down
- Tests benefit from high parallelism even with contention

### DISCARD: Increased parallelism -parallel=16 (Exp 3: 589s — no improvement)
- Default parallelism (GOMAXPROCS=16 on c5.4xlarge) is already optimal

### CRASH: npm_config_prefer_offline=true (Exp 4: instance became unresponsive)
- Likely caused issues with local tarball resolution, hung the test

### DISCARD: node_modules caching via hardlinks (Exp 5: 698s — 21% SLOWER)
- Copying node_modules trees is slow even with hardlinks
- Lock contention around cache adds overhead
- The copy+verify overhead exceeds savings

### KEEP: tmpfs (RAM disk) for test temp dirs (Exp 6: 319s — 44.8% improvement!)
- `sudo mount -t tmpfs -o size=20G tmpfs /mnt/test-tmpfs`
- `export TMPDIR=/mnt/test-tmpfs`
- Massive speedup for npm install and tsc which do heavy file I/O

## Dead Ends
- node_modules hardlink caching: overhead of copying > savings
- npm_config_prefer_offline: causes hangs with local tarballs
- Reducing parallelism: makes everything slower

## OTel Tracing
- Added to `pkg/testing/pulumi-test-language/interface.go`
- Spans: RunLanguageTest, SetupProviders, GenerateSDKs, runLanguageTests, InstallDependencies, GenerateProject, PreviewStack, UpdateStack, PackCoreSdk

## Key Architecture Notes
- 133 test projects with 43 unique dependency combinations
- Each test runs: npm install → tsc (TSC only) → preview → update
- npm install is the biggest bottleneck per test
- Tests run in parallel (default GOMAXPROCS)
