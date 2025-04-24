module l2-resource-config

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-config/sdk/go/v9 v9.0.0
)

replace example.com/pulumi-config/sdk/go/v9 => /ROOT/artifacts/example.com_pulumi-config_sdk_go_v9

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
