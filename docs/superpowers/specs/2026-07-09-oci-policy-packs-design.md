# OCI Policy Packs (`runtime: oci`) — CLI/Engine Design

**Date:** 2026-07-09
**Source:** Option B of "Node-Free Policy Packs: Executable vs OCI — Architecture Overview" (Notion, 2026-07-08).

## Problem

Running a policy pack requires the pack's language toolchain on the machine running
`pulumi up`. A Go/Python/.NET developer needs Node.js and a working `npm install`
solely because their org enforces a TypeScript pack. Execution is also
nondeterministic: no Node version is enforced, and dependencies resolve fresh on
every consumer machine.

Option B makes a pack a digest-pinned container image. The pack runs unchanged
inside the container — the same analyzer gRPC server, announcing its port — and the
engine connects to it instead of booting a language host. The consumer prerequisite
becomes a container runtime instead of a language toolchain.

## Scope

**In scope (this repo, pulumi/pulumi):**

- Engine execution of `runtime: oci` packs, for local `--policy-pack ./dir` and
  server-enforced org packs.
- Two networking modes: host mode (CLI on the host, port mapping) and sibling mode
  (CLI inside a container with a runtime socket — Deployments executor, consumer CI).
- Attach mode: the engine attaches to an already-running pack at a known port
  instead of launching it — the CLI half of the K8s pod-sidecar story, and a local
  debugging affordance. (The service-side pod composition that uses it is separate.)
- `pulumi policy publish` for OCI packs: resolve tag → digest, register the pinned
  ref with the service (no tarball upload).
- `pulumi policy install` for OCI packs (eager image pull).
- API type additions the parallel service work will implement.

**Out of scope / deferred:**

- `pulumi policy new` Dockerfile scaffolding.
- Formal conformance testing at publish (the `GetAnalyzerInfo` boot that publish
  already performs acts as a de facto boot check).
- Private registries and credential brokering (spec's follow-up). Pulls are
  anonymous; auth failures get an actionable error saying private registries are not
  yet supported.
- Service-side work (DB table, API endpoints, Insights evaluator, TF policy check) —
  separate repo, built in parallel against the API shapes below.

## Design decisions (settled)

1. **Integration point:** a third boot path inside `plugin.NewPolicyAnalyzer`
   (`pkg/resource/plugin/analyzer_plugin.go`), alongside the existing legacy-shim and
   language-host paths. Claire's `origin/claire/hackathon-oci-runtime` branch is a
   reference implementation for port scraping, container naming/reaping, and
   cancellation — its `containerHost` decorator structure is not adopted.
2. **Registry access:** shell out to the container runtime CLI (docker, podman, or
   nerdctl). No new Go dependencies (no go-containerregistry).
3. **Manifest carries only `repository`** — the pack's version-free registry home.
   The tag is per-publish input; the digest is publish output, stored service-side.
   No image ref of any kind in the manifest: a digest is only knowable after push
   (edit-per-release churn), and a manifest copy of the ref is a second source of
   truth that can drift from what the service enforces. Note the manifest is not
   baked into the image and the containerized pack never reads it; it is host-side
   pack metadata only.
4. **The digest ref reaches the engine programmatically** (a field on
   `plugin.PolicyAnalyzerOptions`), not via a materialized manifest stub. No fake
   pack directories, no CLI-internal manifest fields.

## Manifest

No parser changes: `ProjectRuntimeInfo` (`sdk/go/common/workspace/project.go`)
already supports the object form.

```yaml
runtime:
  name: oci
  options:
    repository: ghcr.io/acme/policy-packs/security
```

`repository` is required for `runtime: oci`. Validation of its presence happens
where the runtime options are consumed (launcher / publish), not in the generic
manifest loader.

## Image resolution

| Context | Image run |
|---|---|
| Local `--policy-pack ./dir` | `repository:<version from manifest, else latest>`, from the local image cache (the CLI never builds; author builds first). If the image is absent locally, the error states the exact ref the engine looked for and that the author must build and tag it as such. |
| Server-enforced pack | The digest-pinned `ImageRef` from the pack version metadata, passed via `PolicyAnalyzerOptions` |
| Publish | `repository:<tag>` pulled from the registry, then re-referenced by digest |

## Engine execution

### Boot path

In `NewPolicyAnalyzer`:

- **Attach mode first:** if `PULUMI_POLICY_PACK_ATTACH=<pack-name>:<port>[,…]` names
  this pack, skip launching entirely — retry-dial `127.0.0.1:<port>` and wrap the
  connection. Mirrors the existing `PULUMI_DEBUG_PROVIDERS` mechanism for providers
  (`pkg/resource/plugin/provider_plugin.go`). No container lifecycle is managed; the
  failure mode is a dial timeout naming the pack and port. This is the CLI half of
  the K8s pod-sidecar path — the service sets this var on the CLI job container and
  `PULUMI_POLICY_PORT` on each pack sidecar — and doubles as a debugging affordance
  (run a pack under a debugger, point the CLI at it).
- If `PolicyAnalyzerOptions` carries an image ref (server-enforced pack), go straight
  to the OCI launcher — no local manifest is loaded.
- Else load `PulumiPolicy.yaml` as today; if `runtime.name == "oci"`, resolve the
  image from `repository` and go to the launcher.
- Else the existing legacy/language-host paths, unchanged.

The launcher returns a gRPC connection; the analyzer wraps it with the existing
`plugin.NewAnalyzerWithClient` seam. The `AnalyzerHandshake` RPC (engine address,
root/program directories) runs after dialing, as it does for spawned packs.
Everything above the connection — `Analyze`, `AnalyzeStack`, `Configure`, analyzer
caching in `defaultHost.PolicyAnalyzer` — is untouched.

### Container launcher

New package under `pkg/resource/plugin` (policy-analyzer-focused; not a general
"plugins in containers" framework).

**Runtime detection:** `PULUMI_CONTAINER_RUNTIME` env override; else probe PATH for
`docker`, `podman`, `nerdctl` in that order. All registry and container operations
shell out to the selected CLI. No runtime found → loud, actionable error naming the
pack and listing the accepted runtimes.

**Host mode (default — CLI runs directly on the host):**

- Set a fixed in-container port via `-e PULUMI_POLICY_PORT=<P>` and map it with
  `-p 127.0.0.1:0:<P>`; discover the assigned host port via `docker port`; dial
  `127.0.0.1:<hostport>`.
- The pack must reach back to the engine (logging): the engine address in the
  handshake is rewritten to `host.docker.internal:<engineport>`, with
  `--add-host=host.docker.internal:host-gateway` (Docker Desktop, Linux Docker
  ≥ 20.10; podman equivalent).

**Sibling mode (CLI itself inside a container with a runtime socket):**

- Detection: `/.dockerenv` or `/run/.containerenv` present, plus a reachable socket
  or `DOCKER_HOST`; overridable via env.
- Launch with `--network container:<self-id>` (self ID from hostname,
  env-overridable), `PULUMI_POLICY_PORT` set to a free port in the shared namespace;
  dial loopback directly. Engine address passes through unchanged.

**Readiness:** the engine sets `PULUMI_POLICY_PORT` in both modes, so it always
knows where the pack will listen; per the contract, a pack given the port is not
required to announce it. Readiness is therefore gRPC-level retry-dial: retry until
the `AnalyzerHandshake` succeeds (or the channel reaches READY), with a timeout. A
raw TCP connect is not sufficient — Docker's userland proxy binds the host port
before the pack is listening. While retrying, poll the container's state so a
crashed pack fails fast instead of burning the timeout. On failure, the error
includes the container's logs. Attach mode reuses this same retry-dial readiness and
simply skips the launch step — there, log scraping is impossible anyway (the engine
neither launched the pack nor has a runtime CLI) and a dial timeout is the failure.
The stdout port announcement remains part of the pack contract for the case where no
port is assigned, but no mode depends on reading it.

**Lifecycle:** containers run detached with `--rm`, a unique name
(`pulumi-policy-<pack>-<suffix>`), and a `com.pulumi.policy-pack` label. The launcher
registers a close function on the plugin context so the container is stopped on
`Close()`, Ctrl-C, and engine shutdown — mirroring how spawned analyzer processes are
killed.

## API types

Designed against the service's `policy_pack_artifacts` model; the service implements
these in parallel.

- `apitype.RequiredPolicy` gains optional `Runtime string` and `ImageRef string`
  (digest-pinned, `repo@sha256:…`). Absent for legacy packs.
- `apitype.CreatePolicyPackRequest` gains optional `ImageRef string`. When set, the
  service records the ref and returns no presigned upload URL; the client performs no
  tarball upload or completion call.

`ImageRef` is not `repository`: `repository` is the version-free registry home
(manifest, author-written); `ImageRef` is the resolved, digest-pinned reference to
one published version (publish output, service-stored). Consumers only ever see
`ImageRef`.

## Consumer flow (server-enforced pack)

For a `RequiredPolicy` with `ImageRef` set:

- **Install** (`cloudRequiredPolicy.Install`, `pkg/engine/policypacks.go` path):
  eager `pull` of the digest ref — early failure, visible progress, instant container
  start later. No tarball download, no extraction, no dependency install, nothing
  written under `~/.pulumi/policies`.
- **Load:** `loadPolicyPlugins` passes `ImageRef` through the new
  `PolicyAnalyzerOptions` field; the analyzer boots via the launcher.
- `pulumi policy install` behaves the same: pull, done.

## Publish flow

In `cloudPolicyPack.Publish` (`pkg/backend/httpstate/policypack.go`), when the
manifest says `runtime: oci`:

1. Tag = new `--tag` flag on `pulumi policy publish`, defaulting to the pack version.
2. `pull repository:<tag>` — validates what is actually in the registry, not a stale
   local build. The author has already built and pushed with their own tooling.
3. `image inspect` → digest-pinned ref.
3a. **linux/amd64 check:** `manifest inspect` the resolved digest and fail the
   publish if the image (or manifest list) does not include `linux/amd64` — every
   server-side enforcement point (Deployments executor, Insights evaluator, TF
   policy check) runs on it, so an arm64-only image would pass publish and local
   dev, then break org enforcement on consumer machines. Mirrors Option A's
   mandatory linux-amd64 platform.
4. Boot the analyzer from `repository@<digest>` via the launcher; `GetAnalyzerInfo`
   supplies name/version/policy metadata (as publish already does for every runtime)
   and doubles as a boot check.
5. `POST` the create request with metadata + `ImageRef`. No blob upload.

## Error handling

All failures are loud; no silent skips (compliance product):

- No container runtime → names the pack, lists docker/podman/nerdctl, states the
  prerequisite.
- Pull failure → surfaced; auth-shaped failures note that private registries are not
  yet supported.
- Container exits before announcing its port, or readiness timeout → error includes
  container logs.
- Backend rejects `ImageRef` (older service) → the service error is surfaced
  directly.

## Testing

**Unit:**

- Launcher command construction for both networking modes, via a stub `docker`
  script on PATH that records argv and plays back a port line.
- Sibling-mode detection with injected env/filesystem probes.
- Publish client against a mock HTTP backend plus stub runtime.
- Manifest parsing of the `oci` object form.
- Attach mode: start a fake analyzer gRPC server on a port, set
  `PULUMI_POLICY_PACK_ATTACH`, assert the engine attaches without launching anything;
  plus parser tests for the env var format.

**Integration** (skipped when no docker on PATH):

- A testdata policy pack with a Dockerfile (starting from Claire's
  `pkg/oci/smoketest/policy-pack-node`): build the image, run
  `pulumi preview --policy-pack ./dir`, assert a violation is reported and the
  container is removed afterward.
- Cancellation: interrupt mid-run, assert no orphaned container.
