module l2-resource-invoke-dynamic-function

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-any-type-function/sdk/go/v15 v15.0.0
)

replace example.com/pulumi-any-type-function/sdk/go/v15 => /ROOT/projects/l2-resource-invoke-dynamic-function/sdks/any-type-function-15.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
