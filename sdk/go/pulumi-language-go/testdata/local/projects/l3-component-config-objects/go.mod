module l3-component-config-objects

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-primitive/sdk/go/v7 v7.0.0
)

replace example.com/pulumi-primitive/sdk/go/v7 => /ROOT/projects/l3-component-config-objects/sdks/primitive-7.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
