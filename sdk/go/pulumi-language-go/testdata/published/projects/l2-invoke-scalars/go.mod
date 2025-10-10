module l2-invoke-scalars

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-scalar-returns/sdk/go/v21 v21.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-scalar-returns/sdk/go/v21 => /ROOT/artifacts/example.com_pulumi-scalar-returns_sdk_go_v21
