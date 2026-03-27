# Conformance Test Optimization — Autoresearch

## Final Results (53 experiments)

| Metric | Before | After | Improvement |
|---|---|---|---|
| TSC variant (warm) | 578s | 137s avg | **76.2%** |
| All 3 variants (warm) | ~1478s | ~350s | **76.3%** |

## Active Optimizations (17)

### Infrastructure
1. **tmpfs** for test temp dirs
2. **c5.4xlarge** (16 vCPU, 32GB)
3. **Parallel variant execution** (TSC+TSNode+Bun)
4. **GOMAXPROCS=32 + parallel=32** (oversubscription)

### Environment Variables
5. **PULUMI_SKIP_CHECKPOINTS=true** — skip intermediate state saves
6. **PULUMI_NODEJS_TRANSPILE_ONLY=true** — skip ts-node type checking

### Code — Test Framework
7. **Per-test PCL PackageCache** — reuse schemas within test
8. **npm cache warming** with core SDK before tests
9. **Skip installDependencies** for shared-source re-runs
10. **Policy pack install caching** with double-checked locking
11. **Skip local=true variant early**
12. **Cache binary build** (sync.Once)
13. **Inline snapshot edits** — avoid temp directory copies during validation
14. **OTel file-based tracing** + spans

### Code — Language Host
15. **--skipLibCheck** in test tsc, SDK Pack build, and core SDK build
16. **npm flags**: --no-audit --no-fund --no-optional --install-strategy=shallow --legacy-peer-deps
17. **Pack npm flags**: same for SDK packaging

## Bug Fixed
Shared PCL PackageCache caused `l2-resource-option-plugin-download-url` failure.
Fixed with per-test cache.
