# Pulumi dotnet SDK

The Pulumi dotnet SDK lets you write cloud programs in any .NET language.

## !Experimental!

This SDK is currently experimental. Packages are not currently published, and a
number of features are not yet done.

Missing features:

- First class providers
- Provider configs
- Pulumi configs
- Resource reads
- Non-string stack outputs
- Closure serialization

## Installation

```bash
$ dotnet add reference $(go env GOPATH)/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi/Pulumi.csproj
```

This SDK is meant for use with the Pulumi CLI.  Please visit [pulumi.io](https://pulumi.io) for
installation instructions.

## Building and Testing

For anybody who wants to build from source, here is how you do it.

### Prerequisites

This SDK uses [dotnet core](https://github.com/dotnet/core).

At the moment, we only support building on macOS and Linux, where standard GNU tools like `make` are available.

### Make Targets

To build the SDK, simply run `make` from the root directory (where this `README` lives, at `sdk/dotnet/` from the repo's
root).  This will build the code, run tests, and install the package and its supporting artifacts.

Importantly this will generate the C# protobuf bindings under `sdk/dotnet/Pulumi/proto`

At the moment, for local development, we install everything into `/opt/pulumi`.  You will want this on your `$PATH`.