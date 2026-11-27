module l2-invoke-lookup

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-lookup-invoke/sdk/go/v63 v63.0.0
)

replace example.com/pulumi-lookup-invoke/sdk/go/v63 => /ROOT/projects/l2-invoke-lookup/sdks/lookup-invoke-63.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
