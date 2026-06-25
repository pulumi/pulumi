# OCI execution smoke test

Proves the containerized-execution premise end to end. There are two drivers,
one per engine-placement option from `../../../oci-execution-design.md`:

| Driver | Design option | Where the engine runs | How the program reaches it |
|--------|---------------|-----------------------|----------------------------|
| `run.sh` | Option A | in-process with the host CLI | host loopback via `host.docker.internal` |
| `run-pod.sh` | Option C | in a container on a pod network | the engine container's DNS name |

`run-pod.sh` is the real target topology — a self-contained pod with no path
back to the host — so a green run proves pod-network execution by construction.

## What it exercises

- `pulumi-language-oci` (`pkg/cmd/pulumi-language-oci`) — a minimal `oci`
  language host whose `Run()` launches the program as a subprocess or a
  container, rewriting the advertised monitor/engine address for the topology
  (`host.docker.internal`, or the engine container's DNS name).
- The `PULUMI_POD_MODE` bind gate in `rpcutil` — the engine/monitor bind
  `0.0.0.0` so peer containers can reach them.
- A trivial Go program (`program/`) that registers no resources; it just
  connects to the monitor and exports stack outputs (including its hostname).

## Option A — `run.sh` (engine on the host)

Build the CLI and the language host from this branch, put them on `PATH`, then:

```sh
go -C pkg build -o /tmp/ocibin/pulumi ./cmd/pulumi
go -C pkg build -o /tmp/ocibin/pulumi-language-oci ./cmd/pulumi-language-oci
BIN=/tmp/ocibin ./run.sh both       # or: subprocess | container
```

| Stage | `Run()` does | Proves |
|-------|--------------|--------|
| `subprocess` | exec the program binary directly | host discovery + RPC sequence + Run + backend, no networking |
| `container` | `docker run` the program image (`PULUMI_POD_MODE=true`) | the container reaches the host engine via `host.docker.internal` |

## Option C — `run-pod.sh` (engine in a container)

```sh
./run-pod.sh
```

No host `pulumi` is needed — the driver cross-compiles the branch's `pulumi` +
`pulumi-language-oci`, bakes them into an engine image (`Dockerfile.cli`), and
runs `pulumi up` inside a container on a freshly-created pod network. That
container's language host starts the program as a **sibling** container on the
same network, where it dials the engine by container DNS. A green run shows the
program's `hostname` output as a container ID **distinct from the engine
container's name** — proof the two ran as separate containers talking over the
pod network.

## Notes

- Both container paths need a running Docker daemon and build images with
  `docker buildx build`. If your default builder is remote (e.g. Depot), set
  `OCI_BUILDER` to a local builder (defaults to Docker Desktop's
  `desktop-linux`).
- `run-pod.sh` mounts the host Docker socket into the (privileged) engine
  container so its language host can start sibling containers on the same
  daemon. The engine image is built `FROM alpine:3` rather than reusing the
  published `pulumi/pulumi-base`: it must carry *this branch's* `pulumi` (which
  binds `0.0.0.0` in pod mode) plus the prototype `pulumi-language-oci`, and our
  resource-free program needs none of the bundled language runtimes. (For the
  Phase 6 demo, `pulumi-base` is the right base for the *program and provider*
  images, which want the SDK + entrypoint baked in.)
- The program is its own Go module built against the in-repo SDK via a relative
  `replace`; the binary is cross-compiled on the host and `COPY`'d into the
  image, so the `replace` never enters a Docker build.
