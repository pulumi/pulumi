module l2-provider-config-enum

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-config-enum/sdk/go/v41 v41.0.0
)

replace example.com/pulumi-config-enum/sdk/go/v41 => /ROOT/projects/l2-provider-config-enum/sdks/config-enum-41.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
