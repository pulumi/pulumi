module l2-kebab-names

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-kebab-names/sdk/go/v52 v52.0.0
)

replace example.com/pulumi-kebab-names/sdk/go/v52 => /ROOT/projects/l2-kebab-names/sdks/kebab-names-52.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
