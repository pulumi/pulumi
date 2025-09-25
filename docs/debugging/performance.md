# Performance

Pulumi has 2 ways to retrieve performance data during a CLI operation: tracing and
profiles. Both are available via global flags: `--tracing` and `--profiling` respectively.

## Tracing

Tracing provides access to high level execution spans.

```sh
pulumi --tracing file:up.trace up
```

Traces can be viewed directly via a server embedded in Pulumi:

```sh
pulumi view-trace up.trace
```

Traces can also be converted into other formats with `pulumi convert-trace`. Supported
formats include `pprof` and `otel`:

```
pulumi convert-trace up.trace > trace.pprof && go tool pprof -web trace.pprof
```

```
OTEL_SERVICE_NAME=pulumi OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=<endpoint> pulumi convert-trace --otel
```

## Profiles

Profiling provides low level CPU, memory and trace metrics built into Go. This information
can be accessed with [`pprof`](https://github.com/google/pprof) & [`go tool trace`](https://pkg.go.dev/runtime/trace).

```
pulumi --profiling up up
```

You can then access the resulting data with built in Go tools:

```
$ go tool pprof --web up.<pid>.cpu
$ go tool pprof --web up.<pid>.mem
$ go tool trace       up.<pid>.trace
```
