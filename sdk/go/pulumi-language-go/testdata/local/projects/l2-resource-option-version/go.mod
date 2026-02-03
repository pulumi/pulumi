module l2-resource-option-version

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple/sdk/go/v26 v26.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v26 => /ROOT/projects/l2-resource-option-version/sdks/simple-26.0.0
