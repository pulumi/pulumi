module l2-explicit-providers

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component/sdk/go/v13 v13.3.7
)

replace example.com/pulumi-component/sdk/go/v13 => /ROOT/artifacts/example.com_pulumi-component_sdk_go_v13

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
