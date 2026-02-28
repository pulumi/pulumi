# Pulumi Build System Documentation

This document describes the build systems used by the Pulumi monorepo. The repository is transitioning from Makefiles to Bazel. Both systems are maintained in parallel during the transition.

## Repository Structure

```
pulumi/
├── sdk/                          # SDK module (Go 1.24.0)
│   ├── go/                       # Go SDK packages
│   │   └── pulumi-language-go/   # Go language plugin (separate Go module)
│   ├── nodejs/                   # Node.js/TypeScript SDK
│   │   └── cmd/pulumi-language-nodejs/  # Node.js language plugin (Go module)
│   ├── python/                   # Python SDK
│   │   └── cmd/pulumi-language-python/  # Python language plugin (Go module)
│   └── proto/go/                 # Generated Go protobuf stubs
├── pkg/                          # Core CLI module (Go 1.25.0)
│   ├── cmd/pulumi/               # Main CLI binary
│   └── backend/display/wasm/     # WebAssembly display module
├── proto/                        # Protobuf definitions
│   └── pulumi/                   # 18 .proto files
│       ├── codegen/              # Code generation protos
│       └── testing/              # Testing protos
├── tests/                        # Integration tests module
├── build/                        # Make build infrastructure
│   └── common.mk                # Shared Makefile scaffolding
└── scripts/                      # Build and CI scripts
```

## Go Modules

The repository contains 6 Go modules connected by `replace` directives:

| Module | Path | Go Version | Description |
|--------|------|------------|-------------|
| `github.com/pulumi/pulumi/sdk/v3` | `sdk/` | 1.24.0 | Core SDK, proto bindings, Go SDK |
| `github.com/pulumi/pulumi/pkg/v3` | `pkg/` | 1.25.0 | CLI, engine, codegen, backends |
| `github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3` | `sdk/go/pulumi-language-go/` | 1.25.0 | Go language plugin |
| `github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3` | `sdk/nodejs/cmd/pulumi-language-nodejs/` | 1.25.0 | Node.js language plugin |
| `github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3` | `sdk/python/cmd/pulumi-language-python/` | 1.25.0 | Python language plugin |
| `github.com/pulumi/pulumi/tests` | `tests/` | 1.25.0 | Integration tests |

**Dependency graph:** `sdk` ← `pkg` ← `{language plugins, tests}`

## Build Artifacts

| Binary | Source | Build Tags | Description |
|--------|--------|------------|-------------|
| `bin/pulumi` | `pkg/cmd/pulumi` | _(none)_ | Main CLI |
| `bin/pulumi-display.wasm` | `pkg/backend/display/wasm` | `grpcnotrace` | WASM display (GOOS=js GOARCH=wasm) |
| `bin/pulumi-language-go` | `sdk/go/pulumi-language-go` | `grpcnotrace` | Go language host |
| `bin/pulumi-language-nodejs` | `sdk/nodejs/cmd/pulumi-language-nodejs` | `grpcnotrace` | Node.js language host |
| `bin/pulumi-language-python` | `sdk/python/cmd/pulumi-language-python` | `grpcnotrace` | Python language host |

All Go binaries are stamped with `-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}"`.

## Makefile Build System (Legacy)

### Key Files
- `Makefile` - Root orchestration
- `build/common.mk` - Shared infrastructure (targets, Go test flags, sub-project recursion)
- `sdk/go/Makefile` - Go SDK
- `sdk/nodejs/Makefile` - Node.js SDK
- `sdk/python/Makefile` - Python SDK

### Common Targets
```
make build          # Build all (protos, CLI, language plugins, SDKs)
make test_fast      # Fast unit tests
make test_all       # Full test suite including integration
make lint           # Run all linters
make install        # Install to $PULUMI_ROOT (~/.pulumi-dev)
make clean          # Remove build artifacts
```

### Configuration Variables
- `PULUMI_VERSION` - Override version (default: from `scripts/pulumi-version.sh`)
- `PULUMI_ROOT` - Installation directory (default: `$HOME/.pulumi-dev`)
- `GO_TEST_RACE` - Enable race detector (default: `true`)
- `GO_TEST_PARALLELISM` - Test parallelism within packages (default: `10`)
- `GO_BUILD_TAGS` - Default build tags (default: `grpcnotrace`)

## Bazel Build System

> **Status: Phase 0 - Bootstrap** (in progress)

### Key Files
- `MODULE.bazel` - Central dependency and toolchain configuration (bzlmod)
- `.bazelrc` - Build flags and configuration profiles
- `BUILD.bazel` (root) - Gazelle runner and top-level aliases
- `scripts/bazel-workspace-status.sh` - Version stamping
- `.bazelignore` - Directories excluded from Bazel

### Building with Bazel
```
bazel build //...                    # Build everything
bazel build //:pulumi               # Build the CLI
bazel build //:pulumi-language-go   # Build Go language plugin
bazel test //sdk/...                # Test the SDK
bazel test //pkg/...                # Test the pkg module
bazel test //...                    # Run all tests
```

### Architecture

Bazel uses `rules_go` + `gazelle` for Go, `rules_proto` for protobuf, `aspect_rules_js`/`aspect_rules_ts` for Node.js, and `rules_python` for Python.

**Go multi-module handling:** The 6 Go modules with `replace` directives are handled by:
1. `go_deps.from_file()` in MODULE.bazel reads all go.mod files
2. `# gazelle:resolve` directives in BUILD.bazel files map local module imports to Bazel targets
3. External dependencies are resolved via Minimum Version Selection across all modules

**Version stamping:** Uses Bazel's `--workspace_status_command` to inject version, consumed by `x_defs` in `go_binary` rules.

**Protobuf:** Native `proto_library` + language-specific rules replace the Docker-based `proto/generate.sh`.

### Bazel Targets Reference

| Alias | Actual Target | Description |
|-------|---------------|-------------|
| `//:pulumi` | `//pkg/cmd/pulumi` | Main CLI binary |
| `//:pulumi-display.wasm` | `//pkg/backend/display/wasm:pulumi-display` | WASM display module |
| `//:pulumi-language-go` | `//sdk/go/pulumi-language-go` | Go language plugin |
| `//:pulumi-language-nodejs` | `//sdk/nodejs/cmd/pulumi-language-nodejs` | Node.js language plugin |
| `//:pulumi-language-python` | `//sdk/python/cmd/pulumi-language-python` | Python language plugin |

### Protobuf Targets

| Target | Description |
|--------|-------------|
| `//proto/pulumi:pulumi_proto` | Core proto_library |
| `//proto/pulumi:pulumirpc_go` | Go gRPC stubs |
| `//proto/pulumi:pulumirpc_js` | JS/TS gRPC stubs |
| `//proto/pulumi:pulumirpc_python` | Python gRPC stubs |
| `//proto/pulumi/codegen:codegen_proto` | Codegen proto_library |
| `//proto/pulumi/testing:testing_proto` | Testing proto_library |

### SDK Targets

| Target | Description |
|--------|-------------|
| `//sdk/nodejs:sdk_ts` | TypeScript compilation |
| `//sdk/nodejs:unit_tests` | Node.js unit tests |
| `//sdk/python:pulumi_sdk` | Python SDK library |
| `//sdk/python:test_fast` | Python fast tests |

## Protobuf Code Generation

### Proto Files
18 `.proto` files in `proto/pulumi/` define gRPC interfaces for:
- Plugin lifecycle (`plugin.proto`)
- Resource providers (`provider.proto`)
- Language hosts (`language.proto`)
- Deployment engine (`engine.proto`)
- Policy analyzers (`analyzer.proto`)
- Code generation (`codegen/*.proto`)
- Conformance testing (`testing/language.proto`)

### Generated Output
- **Go:** `sdk/proto/go/` - `*.pb.go` and `*_grpc.pb.go` files
- **JavaScript/TypeScript:** `sdk/nodejs/proto/` - `*_pb.js`, `*_pb.d.ts`, `*_grpc_pb.js`, `*_grpc_pb.d.ts`
- **Python:** `sdk/python/lib/pulumi/runtime/proto/` - `*_pb2.py`, `*_pb2_grpc.py`, `*_pb2.pyi`

### Generation Methods
- **Make:** `make build_proto` runs `proto/generate.sh` (Docker-based)
- **Bazel:** `bazel build //proto/...` uses native proto rules

## Node.js SDK

- **Package:** `@pulumi/pulumi`
- **Language:** TypeScript compiled to CommonJS ES2020
- **Package manager:** Yarn 1.x (Makefile) / pnpm (Bazel)
- **Test framework:** Mocha + NYC (coverage)
- **Linting:** ESLint + Biome

## Python SDK

- **Package:** `pulumi` (PyPI)
- **Build backend:** Hatchling
- **Package manager:** uv (Makefile) / pip via rules_python (Bazel)
- **Test framework:** pytest + pytest-xdist
- **Linting:** ruff, mypy, pyright
- **Python support:** 3.10 - 3.14
