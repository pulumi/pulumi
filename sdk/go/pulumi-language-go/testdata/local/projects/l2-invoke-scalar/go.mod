module l2-invoke-scalar

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple-invoke-with-scalar-return/sdk/go/v17 v17.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple-invoke-with-scalar-return/sdk/go/v17 => /ROOT/projects/l2-invoke-scalar/sdks/simple-invoke-with-scalar-return-17.0.0
