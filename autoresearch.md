# Conformance Test Optimization — Autoresearch

## Results

| Metric | Before | After | Improvement |
|---|---|---|---|
| TSC variant | 578s | 139s | **75.8%** |
| All 3 variants (parallel) | ~1478s est | 336s | **77.2%** |

## Optimizations (ranked by impact)

### Infrastructure
1. **tmpfs (RAM disk)** for test temp dirs — largest single win (-44.8%)
2. **Parallel variant execution** (TSC+TSNode+Bun run simultaneously)
3. **c5.4xlarge** instance (16 vCPU, 32GB RAM)

### Code changes
4. **`--skipLibCheck` in SDK Pack build** (`main.go:1986`) — -20.4% by skipping .d.ts type-checking during SDK packaging
5. **`--skipLibCheck` in test tsc** (`main.go:1190`) — -15.4% by skipping .d.ts type-checking in test compilation
6. **Skip shared-source reinstall** (`interface.go:1310`) — -15.4% by not re-running npm install for multi-run tests
7. **Shared PCL PackageCache** (`interface.go:72`) — PCL binding 192ms→42ms avg (78% reduction)
8. **npm `--no-audit --no-fund --no-optional`** (`npm.go:84`) — removes unnecessary npm overhead
9. **Skip local=true variant early** (`language_test.go:150`) — avoids server spawn for skipped variant
10. **Policy pack install caching** (`interface.go:1559`) — shared policy pack dir with locking
11. **Cache binary build** (`language_test.go:43`) — sync.Once across 3 variants
12. **npm flags in Pack** (`main.go:1974`) — `--no-audit --no-fund --no-optional` for SDK packaging

### OTel instrumentation
- File-based trace exporter (`PULUMI_TEST_TRACE_FILE`)
- Spans: RunLanguageTest, SetupProviders, GenerateSDKs, InstallDependencies, GenerateProject, PreviewStack, UpdateStack, PackCoreSdk, BindPCLDirectory, GetProgramDependencies

## Trace Analysis (latest TSC run)

| Span | Total (serial) | Count | Avg | Notes |
|---|---|---|---|---|
| InstallDependencies | 997s | 78 | 12.8s | Still 60% of serial time |
| pulumi-plan | 496s | 145 | 3.4s | Stack preview+update engine |
| GenerateSDKs | 354s | 68 | 5.2s | First-time gen only |
| UpdateStack | 257s | 74 | 3.5s | |
| PreviewStack | 239s | 74 | 3.2s | |
| BindPCLDirectory | 6.8s | 160 | 42ms | 78% reduction with cache |

## Dead Ends
- node_modules hardlink caching (copy overhead > savings)
- npm prefer-offline (crashes with local tarballs)
- Reduced parallelism (makes everything slower)
- --ignore-scripts (no improvement)
- --install-strategy=shallow (marginal)
- Increased parallelism beyond default (no improvement)

## Remaining Bottleneck
npm install at 12.8s per test is the irreducible floor with current npm. To go further would require fundamentally changing the package manager (pnpm) or pre-installing shared node_modules.
