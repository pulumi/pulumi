# Policy Packs as Published Binaries — Design

**Date:** 2026-07-14
**Status:** Approved design, pre-implementation
**Context:** Reframes Option A of the "Node-Free Policy Packs" architecture doc per the
7/14 decision: bring analyzer plugins up to parity with provider plugins — published
binaries. Companion Notion doc: "Node-Free Policy Packs: Executable vs OCI —
Architecture Overview".

## Problem

Running a Pulumi policy pack requires the pack's language toolchain on the consumer
machine. A developer whose program is Go must have Node.js and a working `npm install`
solely because their org enforces a TypeScript policy pack. Execution is also
nondeterministic: no runtime version is enforced and dependencies resolve fresh on
every machine.

The initial goal is Pulumi's official (TypeScript) packs running with zero consumer
prerequisites. The design must have a credible path to third-party publishers and
other languages.

## Model

Analyzer packs align with the provider plugin model: **the published artifact is a
per-platform binary; the artifact has no runtime.** A shipped provider is an
executable entry point named `pulumi-resource-<name>` — what language it was written
in is invisible. The `runtime` concept exists only for source-launched plugins
(`PulumiPlugin.yaml` / `PulumiPolicy.yaml`), governing dev-time and legacy execution.

Consequences:

- **No new runtime.** `PulumiPolicy.yaml` keeps `runtime: nodejs` (etc.) unchanged.
  It describes what the pack is written in and governs source-tree execution (local
  dev, legacy packs). No new manifest fields are required.
- **Binary-ness is a property of the artifact**, chosen at publish by building
  binaries, not declared in YAML.
- **Dispatch is on artifact shape**, the same dispatch `ExecPlugin`
  (`sdk/go/common/resource/plugin/plugin.go:488`) already performs for every plugin
  kind: executable → exec it; source tree → manifest + language host.

Distribution stays **service-hosted** (per-platform blobs behind the Pulumi API), not
provider-style external sources. "Parity with providers" means the exec contract,
handshake, and artifact/entry-point conventions — not the distribution channel.
Rationale: the service schema work is needed either way; external fetch would add
egress/availability/credential surface to every server-side pull site and break
self-hosted/airgapped evaluators; service auth gives org-private packs for free. The
per-version artifact metadata (`location_type`-style column) keeps external hosting
open later without rework.

## Section 1: Artifact model and dispatch

**Manifest: unchanged.**

**Published artifact:** one per platform, a tarball containing a binary named
`pulumi-analyzer-<name>` (provider entry-point convention; platform lives in the blob
key only, never in the artifact).

**Exec contract:** the existing analyzer plugin protocol, unchanged — engine address
as `argv[1]`, gRPC port as first line of stdout, serve `pulumirpc.Analyzer`,
`AnalyzerHandshake` tolerated-if-unimplemented. No protocol, event, or engine
lifecycle changes.

**Self-containedness:** the provider contract guarantees an executable entry point,
not self-containedness (some Node-authored providers ship scripts that exec ambient
`node`). For policy packs self-containedness is the point, so publish conformance
enforces more than "it's executable" (see Section 2). For official JS packs the
blessed recipe is `bun build --compile` (validated by spike: real pack with full
`@pulumi/aws` SDK → ~74MB binary serving Analyze correctly, cross-compiled from one
machine).

**Backwards compatibility, both directions:**

- Old packs, new CLI: legacy source tarballs install and run byte-for-byte as today —
  dispatch falls through to the source-tree branch.
- New packs, old CLI: **dual-publish** — publish uploads the source tarball (as
  today) *and* per-platform binaries. Old CLIs keep requesting the tarball via the
  existing endpoint; new CLIs prefer their platform's binary. No service-side
  CLI-version gating needed.

**Local dev:** `--policy-pack ./dir` uses the same shape dispatch. Source tree →
toolchain, as today. Directory containing a built binary → exec directly.
Build-then-run, symmetric with Go provider development. Dispatch checks for
`pulumi-analyzer-<name>` first, then the host platform's conventional path
`bin/pulumi-analyzer-<name>-<os>-<arch>[.exe]`, so a fresh CI-convention build tree
runs without renaming.

## Section 2: Publish flow

**Command:** `pulumi policy publish` — same command; mode determined by whether
`--binary` flags are passed.

**Binary opt-in — explicit flags only, no discovery:**

- Repeated `--binary <os>-<arch>=<path>` flags, each pointing at a pack-relative
  path built by the author's CI.
- No `--binary` flags → today's source publish, unchanged. Existing packs and CI
  pipelines keep working with zero changes until an author opts in. There is no
  publish-time convention scan of `bin/`.

**Validation at publish (binaries present):**

1. **`linux-amd64` is mandatory** — the Deployments executor and both server-side
   evaluators run there. Hard error, not a warning.
2. **Conformance boot:** exec the host-platform binary (required to be among the
   built platforms), perform the analyzer handshake, call `GetAnalyzerInfo`, run a
   synthetic `Analyze`. Bad binaries die at publish, never on a consumer machine.
   `GetAnalyzerInfo` from the binary is the source of policy metadata sent to the
   service — the metadata provably describes the shipped artifact.
3. Version/name consistency between `GetAnalyzerInfo` and `PulumiPolicy.yaml`, as
   today.

**Upload — dual-publish:** source tarball exactly as today (`npm pack` / TGZ), plus
each platform binary as a tarball via per-platform presigned upload URIs.

**API changes (`sdk/go/common/apitype/policy.go`; service implementation is a
dependency):**

- `CreatePolicyPackRequest` gains `platforms []string` (omitempty).
- `CreatePolicyPackResponse` gains `platformUploadURIs map[string]string` alongside
  the existing `UploadURI`. Old service ignores the request field and returns only
  `UploadURI`; the CLI then fails loudly ("service does not support binary packs"),
  with re-running without `--binary` as the workaround. No silent fallback.
- `RequiredPolicy` gains per-platform locations (or the download endpoint takes a
  platform param) — consumed in Section 3.

## Section 3: Consumer install and engine exec

**Download resolution.** `EnsurePoliciesAreInstalled` → `installRequiredPolicy` asks
for the consumer's platform first. Via the extended `RequiredPolicy`, the service
returns the binary artifact for `<os>-<arch>` if the version was binary-published and
has that platform, else the source tarball. A binary-published pack missing the
consumer's platform falls back to source **loudly** (warning naming the missing
platform). New CLI + old service: per-platform fields absent → use `PackLocation` as
today.

**Install** into `~/.pulumi/policies/<org>/pulumi-analyzer-<name>-v<ver>/` (org
scoping is correct by construction — pack names are only unique per org, unlike the
global `~/.pulumi/plugins/` namespace):

- Binary artifact → extract + `chmod +x`. Dependency install skipped entirely — no
  npm, no venv, no toolchain probe.
- Source tarball → today's path unchanged (extract, `InstallDependencies`).

**Engine exec — shape dispatch in `NewPolicyAnalyzer`**
(`sdk/go/common/resource/plugin/analyzer_plugin.go:104`). Before the current
runtime-resolution logic, check the pack directory for an executable
`pulumi-analyzer-<name>`:

- Present (or the host-platform conventional path exists — see Local dev in
  Section 1) → `newPlugin(..., AnalyzerPlugin, []string{host.ServerAddr(), "."},
  ...)` — the launch the modern path already uses, minus language-host resolution.
  Existing handshake, `ConfigureStack`, everything downstream unchanged.
- Absent → today's logic untouched (language plugin present → runtime launch; else
  legacy `pulumi-analyzer-policy-<runtime>` shim).

**Server-side evaluators (service repo, dependency):** Insights evaluator and TF
policy check download the `linux-amd64` binary artifact and exec it; their install
switch gains a skip-dependency-install case; the current silent skip of unrecognized
shapes becomes a loud failure. Deployments needs nothing — the executor runs the CLI.

## Section 4: Testing and rollout

**Unit:** binary flag parsing (`--binary` overrides, `.exe`, linux-amd64
enforcement), publish mode switch, apitype round-trips, shape dispatch
(binary present / absent / non-executable file).

**Integration:** a test analyzer written in Go (compiled in-test; no bun/Node in CI)
serving a minimal `Analyzer`, exercising publish conformance boot,
install-extract-exec against the mock service backend, and `--policy-pack <builtdir>`
local exec. Existing policy integration tests in `tests/` run unmodified against
source packs — they are the backwards-compat suite.

**Cross-version:** old-shaped service response (no per-platform fields) yields
today's behavior exactly.

**What lands where:**

1. **pulumi/pulumi (this design's scope):** apitype additions, publish
   flag-parsing/validation/upload, install resolution, engine dispatch. Mergeable and
   testable against a mock before any service work — all changes are inert until the
   service returns the new fields.
2. **Service repo (dependency):** artifacts table (per the Notion doc's
   `policy_pack_artifacts` sketch), per-platform upload URIs, platform-aware download
   resolution, evaluator/TF-check changes, loud failure on unknown artifact shapes.
3. **Official pack repos:** per-platform `bun build --compile` CI steps producing
   `bin/pulumi-analyzer-<name>-<os>-<arch>`; republish. Dual-publish lets each repo
   migrate independently.

**Rollout order:** CLI ships first (dormant) → service → official packs one at a time
→ consumers on new CLIs stop needing Node. Old CLIs consume the source tarball
indefinitely; dropping dual-publish is a per-pack decision far later.

**Risks:** binary size (~75–110MB per platform per version) is a service storage/cost
question, already flagged in the Notion doc. Bun compile fidelity for packs with
native deps is validated per pack by publish conformance, not assumed.
