module l2-secret-unknown

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-output/sdk/go/v23 v23.0.0
)

replace example.com/pulumi-output/sdk/go/v23 => /ROOT/projects/l2-secret-unknown/sdks/output-23.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
