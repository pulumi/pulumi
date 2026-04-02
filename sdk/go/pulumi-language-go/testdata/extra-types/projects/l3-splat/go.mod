module l3-splat

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-nestedobject/sdk/go v1.42.0
)

replace example.com/pulumi-nestedobject/sdk/go => /ROOT/artifacts/example.com_pulumi-nestedobject_sdk_go

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
