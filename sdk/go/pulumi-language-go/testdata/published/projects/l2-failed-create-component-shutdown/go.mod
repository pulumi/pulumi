module l2-failed-create-component-shutdown

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-failingcomponent/sdk/go/v59 v59.0.0
)

replace example.com/pulumi-failingcomponent/sdk/go/v59 => /ROOT/artifacts/example.com_pulumi-failingcomponent_sdk_go_v59

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
