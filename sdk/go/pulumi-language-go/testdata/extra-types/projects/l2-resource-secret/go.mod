module l2-resource-secret

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-secret/sdk/go/v14 v14.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-secret/sdk/go/v14 => /ROOT/artifacts/example.com_pulumi-secret_sdk_go_v14
