module l2-union

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-union/sdk/go/v18 v18.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-union/sdk/go/v18 => /ROOT/artifacts/example.com_pulumi-union_sdk_go_v18
