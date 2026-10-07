module l3-component-invoke

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-config/sdk/go/v9 v9.0.0
	example.com/pulumi-multi-argument-invoke/sdk/go/v44 v44.0.0
)

replace example.com/pulumi-config/sdk/go/v9 => /ROOT/projects/l3-component-invoke/sdks/config-9.0.0

replace example.com/pulumi-multi-argument-invoke/sdk/go/v44 => /ROOT/projects/l3-component-invoke/sdks/multi-argument-invoke-44.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
