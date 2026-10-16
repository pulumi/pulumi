module l2-resource-const

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-constant/sdk/go/v43 v43.0.0
)

replace example.com/pulumi-constant/sdk/go/v43 => /ROOT/artifacts/example.com_pulumi-constant_sdk_go_v43

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace github.com/pulumi/pulumi/shared/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_shared_v3
