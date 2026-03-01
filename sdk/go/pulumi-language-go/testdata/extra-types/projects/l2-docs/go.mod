module l2-docs

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-docs/sdk/go/v28 v28.0.0
)

replace example.com/pulumi-docs/sdk/go/v28 => /ROOT/artifacts/example.com_pulumi-docs_sdk_go_v28

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
