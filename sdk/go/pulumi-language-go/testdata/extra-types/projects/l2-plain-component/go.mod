module l2-plain-component

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-plaincomponent/sdk/go/v36 v36.0.0
)

replace example.com/pulumi-plaincomponent/sdk/go/v36 => /ROOT/artifacts/example.com_pulumi-plaincomponent_sdk_go_v36

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
