module l2-invoke-scalar

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple-invoke-with-scalar-return/sdk/go/v17 v17.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple-invoke-with-scalar-return/sdk/go/v17 => /ROOT/artifacts/example.com_pulumi-simple-invoke-with-scalar-return_sdk_go_v17
