# OCI execution smoke test

Proves the containerized-execution premise end to end (design Option A: the
engine runs in-process with the host CLI, and the program reaches it over
loopback). See `../../../oci-execution-design.md` for the full design.

## What it exercises

- `pulumi-language-oci` (`pkg/cmd/pulumi-language-oci`) — a minimal `oci`
  language host whose `Run()` launches the program as either a subprocess or a
  container.
- The `PULUMI_POD_MODE` bind gate in `rpcutil` — the engine/monitor bind
  `0.0.0.0` so a container can reach them.
- A trivial Go program (`program/`) that registers no resources; it just
  connects to the monitor and exports stack outputs.

## Running it

Build the CLI and the language host from this branch, put them on `PATH`, then:

```sh
go -C pkg build -o /tmp/ocibin/pulumi ./cmd/pulumi
go -C pkg build -o /tmp/ocibin/pulumi-language-oci ./cmd/pulumi-language-oci
BIN=/tmp/ocibin ./run.sh both       # or: subprocess | container
```

The driver runs `pulumi up` in two stages to isolate failure domains:

| Stage | `Run()` does | Proves |
|-------|--------------|--------|
| `subprocess` | exec the program binary directly | host discovery + RPC sequence + Run + backend, no networking |
| `container` | `docker run` the program image (`PULUMI_POD_MODE=true`) | the container reaches the host engine via `host.docker.internal` |

A passing container stage shows a **container ID** as the `hostname` output
(vs. the host's name in the subprocess stage) — the program really ran in a
container and talked back to the host engine.

## Notes

- The container stage needs a running Docker daemon and builds the program image
  from the `Dockerfile` with `docker build`. If your default buildx builder is
  remote (e.g. Depot), set `OCI_BUILDER` to a local builder (defaults to Docker
  Desktop's `desktop-linux`).
- The program is its own Go module built against the in-repo SDK via a relative
  `replace`; the binary is cross-compiled on the host and `COPY`'d into the
  image, so the `replace` never enters a Docker build.
