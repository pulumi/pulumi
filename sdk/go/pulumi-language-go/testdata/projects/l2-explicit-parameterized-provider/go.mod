module l2-explicit-parameterized-provider

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-goodbye/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-goodbye/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-goodbye_sdk_go_v2

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
