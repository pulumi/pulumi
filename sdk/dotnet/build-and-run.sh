#!/bin/bash
set -eou pipefail
IFS="\n\t"

GOBIN=/opt/pulumi/bin go install ./cmd/pulumi-language-dotnet
dotnet build .
dotnet publish Pulumi.Host/pulumi-language-dotnet-exec.csproj
export PATH=/opt/pulumi/bin:$(go env GOPATH)/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi.Host/bin/Debug/netcoreapp2.0/publish:$PATH
cd examples
pulumi update  --diff --yes

