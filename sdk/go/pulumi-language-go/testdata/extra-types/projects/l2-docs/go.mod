module l2-docs

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-docs/sdk/go/v25 v25.0.0
)

replace example.com/pulumi-docs/sdk/go/v25 => /ROOT/artifacts/example.com_pulumi-docs_sdk_go_v25

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
