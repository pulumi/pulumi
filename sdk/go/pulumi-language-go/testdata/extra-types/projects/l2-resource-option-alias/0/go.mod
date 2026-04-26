module l2-resource-option-alias

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component/sdk/go/v13 v13.3.7
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-component/sdk/go/v13 => /ROOT/artifacts/example.com_pulumi-component_sdk_go_v13

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-simple_sdk_go_v2
