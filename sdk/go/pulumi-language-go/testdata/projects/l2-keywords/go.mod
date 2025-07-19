module l2-keywords

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-keywords/sdk/go/v20 v20.0.0
)

replace example.com/pulumi-keywords/sdk/go/v20 => /ROOT/artifacts/example.com_pulumi-keywords_sdk_go_v20

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
