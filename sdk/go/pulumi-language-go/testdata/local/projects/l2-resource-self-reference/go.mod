module l2-resource-self-reference

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-selfref/sdk/go v1.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-selfref/sdk/go => /ROOT/projects/l2-resource-self-reference/sdks/selfref-1.0.0
