module l2-index-mod

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-index-mod/sdk/go/v35 v35.0.0
)

replace example.com/pulumi-index-mod/sdk/go/v35 => /ROOT/projects/l2-index-mod/sdks/index-mod-35.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
