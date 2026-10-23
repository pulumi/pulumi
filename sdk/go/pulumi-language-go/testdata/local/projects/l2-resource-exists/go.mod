module l2-resource-exists

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-exists/sdk/go/v59 v59.0.0
)

replace example.com/pulumi-exists/sdk/go/v59 => /ROOT/projects/l2-resource-exists/sdks/exists-59.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
