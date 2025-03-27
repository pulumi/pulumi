module l2-invoke-secrets

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
	example.com/pulumi-simple-invoke/sdk/go/v10 v10.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-simple_sdk_go_v2

replace example.com/pulumi-simple-invoke/sdk/go/v10 => /ROOT/artifacts/example.com_pulumi-simple-invoke_sdk_go_v10
