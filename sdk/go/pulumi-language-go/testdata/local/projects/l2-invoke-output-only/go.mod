module l2-invoke-output-only

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-output-only-invoke/sdk/go/v24 v24.0.0
)

replace example.com/pulumi-output-only-invoke/sdk/go/v24 => /ROOT/projects/l2-invoke-output-only/sdks/output-only-invoke-24.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
