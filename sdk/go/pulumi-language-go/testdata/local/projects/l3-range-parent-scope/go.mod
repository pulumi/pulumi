module l3-range-parent-scope

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-nestedobject/sdk/go v1.42.0
)

replace example.com/pulumi-nestedobject/sdk/go => /ROOT/projects/l3-range-parent-scope/sdks/nestedobject-1.42.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
