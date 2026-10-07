module l2-invoke-multi-argument

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-multi-argument-invoke/sdk/go/v44 v44.0.0
)

replace example.com/pulumi-multi-argument-invoke/sdk/go/v44 => /ROOT/artifacts/example.com_pulumi-multi-argument-invoke_sdk_go_v44

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
