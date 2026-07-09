# Executable Policy Packs (`runtime: executable`) — pulumi/pulumi Design

**Date:** 2026-07-09
**Source:** Option A of the architecture doc "Node-Free Policy Packs: Executable vs OCI"
(Notion: `397fdbdf1cce81499020c05402908f18`).

## Problem

Running a policy pack today requires the pack's language toolchain on the consumer
machine: the CLI downloads a source tarball, runs `npm install`/pip, and boots the pack
through a language host. A Go developer whose org enforces a TypeScript pack needs
Node.js solely for that. Execution is also nondeterministic — no runtime version is
pinned and dependencies resolve fresh per machine.

This design introduces a new self-describing pack type, `runtime: executable`: a pack is
a set of self-contained per-platform binaries, and the CLI execs the one matching the
host platform. No toolchain, no dependency install, ever. The pack type is chosen once
at publish; there is no runtime selection, capability probing, or fallback. Legacy
`nodejs`/`python`/`opa` packs keep today's exact path, untouched.

## Scope

Everything in the pulumi/pulumi repo: manifest handling, engine boot, local
`--policy-pack` execution, download/install, `pulumi policy publish` (conformance gate +
per-platform uploads), and the `apitype` wire contract the service will implement.

**Out of scope** (separate efforts): the service implementation (API handlers,
`policy_pack_artifacts` table), Insights evaluator and TF policy check changes, the
executable pack template in `pulumi/templates-policy`, official pack repo CI (Bun
builds), and a standalone `pulumi policy validate` command (deferred; internals are
factored so it can wrap them later).

## Decisions already made

- **Contract:** the existing analyzer plugin protocol. The binary serves
  `pulumirpc.Analyzer` gRPC on loopback, receives the engine address as `argv[1]`,
  prints its port as the first line of stdout, and runs until killed. No CLI compile
  command — authors build with their own tooling.
- **Service dependency:** this repo defines the new `apitype` contract; publishing
  against a service that doesn't support it fails loudly. No feature flag, no
  fat-tarball fallback.
- **Conformance:** runs inside `pulumi policy publish` only (no standalone command in
  v1). `linux-amd64` is mandatory in every executable pack (server-side evaluators are
  linux-amd64); other platforms optional. Conformance boots the host platform's binary,
  so the host platform must be declared or publish fails.
- **Artifact format:** one gzipped tarball per platform containing `PulumiPolicy.yaml`
  plus that platform's binary, nested under the existing `package/` directory
  convention.
- **Download shape:** the platform-keyed download map travels to the CLI in
  `apitype.RequiredPolicy`; the client picks its own platform.

## Section 1: Manifest and engine boot (local execution)

### Manifest

No parser changes: `ProjectRuntimeInfo` (`sdk/go/common/workspace/project.go`) already
supports the object form with `options`.

```yaml
runtime:
  name: executable
  options:
    binaries:
      linux-amd64: bin/pack-linux-amd64
      linux-arm64: bin/pack-linux-arm64
      darwin-arm64: bin/pack-darwin-arm64
      windows-amd64: bin/pack-windows-amd64.exe
```

Add a typed accessor in `sdk/go/common/workspace` that extracts and validates the
`binaries` map from runtime options:

- Platform keys must be `<os>-<arch>` with known GOOS/GOARCH values.
- Paths must be relative and resolve inside the pack directory (reject absolute paths
  and `../` escapes).
- The map must be non-empty.

`linux-amd64` presence is enforced at publish, not at load — a developer on a Mac
iterating locally does not need the linux binary.

### Engine boot

In `plugin.NewPolicyAnalyzer` (`sdk/go/common/resource/plugin/analyzer_plugin.go`), add
a third boot branch ahead of the existing language-host and legacy-shim
(`pulumi-analyzer-policy-<runtime>`) modes: when `proj.Runtime.Name() == "executable"`,

1. Look up `runtime.GOOS-runtime.GOARCH` in the validated binaries map. Missing entry →
   loud, actionable error naming the pack, the host platform, and the platforms the
   pack does declare.
2. Exec the binary via the existing `newPlugin` machinery
   (`sdk/go/common/resource/plugin/plugin.go`) with args `[host.ServerAddr()]` — the
   same exec, first-stdout-line port handshake, and gRPC dial used by analyzer plugins
   today.
3. Environment comes from the existing `constructEnv` (config, stack metadata), minus
   nodejs special-casing.

`ConfigureStack` and all downstream analyzer calls are unchanged — the pack is just
another `pulumirpc.Analyzer` client.

### Local dev

`--policy-pack ./dir` works with no additional changes once boot works:
`LoadLocalPolicyPackAnalyzers` → `Host.PolicyAnalyzer` → new branch. Build-first
workflow, symmetric with Go provider development; the CLI never compiles anything.

## Section 2: Install and download (consumer path)

### Artifact format

Each platform's artifact is a `.tar.gz` containing exactly `PulumiPolicy.yaml` and that
platform's binary — no sources, no other platforms' binaries — nested under `package/`
so the existing extraction code works unchanged. The tarball preserves the executable
bit; on non-Windows platforms extraction additionally `chmod +x`es the declared binary
defensively.

The `binaries` map inside the shipped `PulumiPolicy.yaml` stays complete (all declared
platforms) even though only one binary is present: boot only looks up the host
platform's key, and rewriting the manifest per platform would add publish complexity for
no benefit.

### Download

`apitype.RequiredPolicy` gains an optional field alongside the existing `PackLocation`:

```go
// PackLocations maps platform ("linux-amd64", …) to a download URL for
// that platform's artifact. Set only for packs with per-platform artifacts.
PackLocations map[string]string `json:"packLocations,omitempty"`
```

`cloudRequiredPolicy.Download` (`pkg/backend/httpstate/policypack.go`) picks
`runtime.GOOS-runtime.GOARCH` from the map when present. A missing host platform is a
loud error listing the platforms the pack does provide — never a silent skip. Legacy
packs continue to use `PackLocation` untouched.

The mandatory `linux-amd64` artifact is what the server-side evaluators (Insights
evaluator, TF policy check, Deployments executor) will download through this same
`PackLocations` contract — that is why the platform is required at publish.

### Install

`installRequiredPolicy` (`pkg/backend/httpstate/policypack.go`) keeps its
ExtractTGZ → rename → reload-manifest flow. For `runtime: executable` it stops after the
manifest reload: no language runtime lookup, no `InstallDependencies`, just the chmod.
Install location and naming (`~/.pulumi/policies/<org>/pulumi-analyzer-<name>-v<version>/`)
are unchanged, so `Installed()` checks and the install manager need no modification.

## Section 3: Publish

### Flow

`cloudPolicyPack.Publish` (`pkg/backend/httpstate/policypack.go`) gets an executable
branch replacing the boot-and-tarball steps:

1. **Validate the matrix.** Load the `binaries` map; require `linux-amd64` declared;
   require every declared file to exist in the pack dir; require the host platform to
   be declared (conformance cannot run otherwise — the error says exactly that).
2. **Conformance.** Boot the host-platform binary through the same `NewPolicyAnalyzer`
   path consumers use — publish exercises the real path, no separate boot code. Then:
   - `GetAnalyzerInfo` — also the source of name/version/policy metadata, as today.
   - A synthetic `Analyze` call with a fabricated resource; the assertion is
     transport-level only (the call completes without a gRPC/serialization error);
     returned diagnostics are irrelevant.
   Any failure aborts publish with the binary's stderr attached.
3. **Build per-platform tarballs** (Section 2 format) into a temp dir.
4. **Upload.** PUT each tarball to its presigned URL (new response shape below), then
   the unchanged publish-complete call.

Conformance internals are factored as a helper (in `pkg/backend/httpstate` or
`pkg/engine`) so a future `pulumi policy validate` command can wrap them.

Caveat, verified empirically: conformance boots the binary in the author's pack
directory, so a binary that silently depends on files outside itself — e.g. a
bun-compiled pack whose `@pulumi/pulumi` falls back to `require`-ing a vendored
`typescript.js` from `node_modules` by absolute path — passes conformance while
`node_modules` is present and only fails on consumer machines. The publish gate
catches this class of defect only when the build residue is absent at publish
time. The official pack template must pin `typescript` as an explicit
dependency (which lets bun bundle it statically) and pass the pack version
explicitly via `PolicyPackArgs.version` (pulumi-policy#452) with a static
`package.json` import, and pack-repo CI should remove `node_modules` between
build and publish so conformance boots the binary as a consumer would.

### apitype

- `CreatePolicyPackRequest` gains `Platforms []string` — the declared matrix; empty
  means a legacy single-tarball pack.
- `CreatePolicyPackResponse` gains:

```go
// PlatformUploadURIs maps platform to a presigned upload for that
// platform's artifact. Set when the request declared Platforms.
PlatformUploadURIs map[string]PolicyPackUpload `json:"platformUploadURIs,omitempty"`

type PolicyPackUpload struct {
    UploadURI       string            `json:"uploadURI"`
    RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`
}
```

Existing `UploadURI`/`RequiredHeaders` remain for legacy packs. An old service ignores
`Platforms` and returns no `PlatformUploadURIs`; the CLI treats that as "service does
not support executable policy packs" and fails with an explicit error naming the
minimum service requirement.

The `pulumi policy publish` command needs no new flags — the runtime name in the
manifest drives everything.

## Section 4: Error handling

The recurring requirement is *loud and actionable* — this is a compliance feature, and
silent skips are unacceptable.

- **Boot, missing platform:** "policy pack `<name>` does not provide a binary for
  `darwin-arm64`; it supports: linux-amd64, windows-amd64. The pack must be republished
  with a darwin-arm64 binary to run on this machine."
- **Boot, binary fails to start or bad handshake:** surface process stderr and exit
  code through the existing plugin error plumbing. The nodejs-specific
  `errRunPolicyModuleNotFound` npm hint must not fire for executable packs.
- **Malformed `binaries` map** (unknown platform key, absolute/escaping path, empty
  map): load-time error naming the offending key.
- **Download, no artifact for host platform:** loud error listing available platforms.
- **Publish against old service:** explicit "this service version does not support
  executable policy packs", not a nil-map crash.

## Section 5: Testing

- **Unit:**
  - Binaries-map parsing/validation table tests in `sdk/go/common/workspace`.
  - Boot-branch selection tests in `analyzer_plugin.go` (executable vs language-host vs
    legacy shim).
  - Install short-circuit test (no `InstallDependencies` call for executable packs).
  - Publish matrix-validation tests (missing linux-amd64, missing file, undeclared host
    platform).
  - apitype JSON round-trip tests.
- **Engine/integration:**
  - A small Go test analyzer binary, built by the test itself (same pattern as existing
    plugin tests), serving a canned `pulumirpc.Analyzer`.
  - End-to-end: `--policy-pack ./dir` with `runtime: executable` boots it and returns
    diagnostics; a manifest missing the host platform produces the documented error.
  - Publish path against a mock HTTP backend asserting per-platform PUTs (existing
    `client` test patterns).
- **Not covered here:** Bun-compiled real packs (official pack repos' CI) and
  conformance against the real service.

## Alternatives considered

- **Model the binary as an analyzer plugin resolved via `workspace.GetPluginPath`:**
  reuses legacy shim resolution but contorts plugin-path resolution for a binary that
  lives in the pack dir, and the platform map doesn't fit that model. Rejected.
- **An "executable" language host plugin:** uniform single boot path but adds a
  pointless intermediary process and a new shipped binary. Rejected.
- **Fat-tarball fallback for old services:** works without service changes but bloats
  every download to the full matrix (~400MB) and contradicts the per-platform goal.
  Rejected in favor of failing loudly.
