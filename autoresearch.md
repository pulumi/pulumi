# Conformance Test Optimization — Autoresearch

## Results (38 experiments)

| Metric | Before | After | Improvement |
|---|---|---|---|
| TSC variant | 578s | ~200s | **65.4%** |
| All 3 variants (parallel) | ~1478s est | ~450s | **~70%** |

## Active Optimizations (13)

### Infrastructure
1. **tmpfs (RAM disk)** for test temp dirs — largest single win
2. **c5.4xlarge** instance (16 vCPU, 32GB RAM)
3. **Parallel variant execution** (TSC+TSNode+Bun simultaneously)

### Code — Test Framework (`pkg/testing/pulumi-test-language/`)
4. **Per-test PCL PackageCache** — reuse loaded schemas within same test
5. **Pre-warm npm cache** with core SDK install before tests
6. **Skip installDependencies for shared-source re-runs**
7. **Policy pack install caching** with double-checked locking
8. **Skip local=true variant early** — avoid server spawn for skipped variant
9. **Cache binary build** (`sync.Once`) across 3 variants
10. **OTel file-based trace exporter** + comprehensive span instrumentation

### Code — Language Host (`sdk/nodejs/`)
11. **`--skipLibCheck`** in test tsc, SDK Pack build, and core SDK build
12. **npm flags**: `--no-audit --no-fund --no-optional --install-strategy=shallow --legacy-peer-deps`
13. **Pack npm flags**: same optimization flags for SDK packaging npm install

## Trace-Based Time Breakdown (TSC, 2280s serial across 130 tests)

| Phase | Serial Time | % | Effective (16 parallel) |
|---|---|---|---|
| InstallDependencies | 1044s | 45.8% | ~65s |
| GenerateSDKs | 506s | 22.2% | ~32s |
| PreviewStack | 323s | 14.2% | ~20s |
| UpdateStack | 286s | 12.5% | ~18s |
| SetupProviders | 105s | 4.6% | ~7s |
| Other | 16s | 0.7% | ~1s |
| **Total** | **2280s** | | **~143s + critical path** |

## Dead Ends (25 experiments discarded)
- node_modules hardlink/symlink/cp caching (npm re-resolves anyway)
- npm prefer-offline (incompatible with local tarballs)
- Reduced/increased parallelism (default is near-optimal)
- npm cache on tmpfs (cold cache worse than warm disk)
- bun wrapper (lock file incompatibility with GetProgramDependencies)
- Pre-seeded node_modules (npm ignores existing and re-resolves)
- --ignore-scripts, --no-package-lock (breaks dependency validation)
- Shared PackageCache (correctness bug: version conflicts across tests)

## Bug Fixed
- Shared PCL PackageCache caused `l2-resource-option-plugin-download-url` failure
  (simple@2.0.0 cached instead of simple@27.0.0). Fixed with per-test cache.

## Critical Path Analysis
- `l1-builtin-require-pulumi-version`: 84-155s (2 runs, PreviewStack=38s for bail)
- Component provider tests: 34-68s (provider npm install in SetupProviders)
- Policy tests: 50-100s (policy pack npm install)
