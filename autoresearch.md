# Conformance Test Optimization — Autoresearch

## Final Results (46 experiments)

| Metric | Before | After | Improvement |
|---|---|---|---|
| TSC variant (warm) | 578s | 177s avg | **69.4%** |
| TSC variant (cold) | 578s | 209s | **63.9%** |
| All 3 variants (warm) | ~1478s | 502s | **66.0%** |
| All 3 variants (cold) | ~1478s | 563s | **61.9%** |

## Active Optimizations (14)

### Infrastructure
1. **tmpfs (RAM disk)** for test temp dirs
2. **c5.4xlarge** (16 vCPU, 32GB)
3. **Parallel variant execution** (TSC+TSNode+Bun simultaneously)
4. **GOMAXPROCS=32 + parallel=32** (oversubscription works with tmpfs)

### Code — Test Framework
5. **Per-test PCL PackageCache** — reuse loaded schemas within test
6. **npm cache warming** with core SDK + typescript + @types/node
7. **Skip installDependencies** for shared-source re-runs
8. **Policy pack install caching** with double-checked locking
9. **Skip local=true variant early**
10. **Cache binary build** (sync.Once across variants)
11. **OTel file-based tracing** + comprehensive span instrumentation
12. **TestRunIteration spans** for per-run visibility

### Code — Language Host
13. **`--skipLibCheck`** in test tsc, SDK Pack build, and core SDK build
14. **npm flags**: `--no-audit --no-fund --no-optional --install-strategy=shallow --legacy-peer-deps` (program + Pack)

## Trace-Based Time Breakdown

| Phase | Serial Time | % | Avg/call |
|---|---|---|---|
| InstallDependencies | 1044s | 45.8% | 12.9s |
| GenerateSDKs | 506s | 22.2% | 6.9s |
| PreviewStack | 323s | 14.2% | 4.1s |
| UpdateStack | 286s | 12.5% | 3.7s |
| SetupProviders | 105s | 4.6% | 0.8s |
| Other | 16s | 0.7% | — |

## Critical Path
- `l1-builtin-require-pulumi-version`: 84-185s (2 runs, bail path=38s/op)
- Engine plugin lifecycle causes 38s per bail operation (5s shutdown timeout + gRPC graceful stop)
- This single test accounts for ~50% of wall time variance

## Dead Ends (32 experiments discarded)
- node_modules hardlink/symlink/cp caching (npm re-resolves anyway)
- npm prefer-offline (incompatible with local tarballs)
- Shared PCL PackageCache (**bug**: version conflicts across tests)
- DISABLE_AUTOMATIC_PLUGIN_ACQUISITION (causes plugin errors)
- Reduced parallelism (slower), increased beyond 32 (OOM)
- npm cache on tmpfs (cold worse than warm disk)
- bun wrapper (lock file incompatibility)
- --ignore-scripts, --no-package-lock (breaks validation)
- OOM sniffer timeout reduction (not the cause)
- npm workspaces (incompatible with test isolation needs)
- Pre-seeded node_modules (npm ignores existing)
- Test shuffling (no impact on critical path)

## Bug Fixed
Shared PCL PackageCache caused `l2-resource-option-plugin-download-url` to fail
(simple@2.0.0 cached instead of simple@27.0.0). Fixed with per-test cache.
