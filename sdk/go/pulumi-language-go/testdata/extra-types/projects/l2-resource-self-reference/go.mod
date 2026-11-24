module l2-resource-self-reference

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-selfref/sdk/go/v54 v54.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-selfref/sdk/go/v54 => /ROOT/artifacts/example.com_pulumi-selfref_sdk_go_v54
