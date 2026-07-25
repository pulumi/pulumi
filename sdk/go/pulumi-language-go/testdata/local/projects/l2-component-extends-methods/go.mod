module l2-component-extends-methods

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-inherit/sdk/go v1.0.0
)

replace example.com/pulumi-inherit/sdk/go => /ROOT/projects/l2-component-extends-methods/sdks/inherit-1.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
