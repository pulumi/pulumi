module l2-resource-option-parent

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple/sdk/go/v27 v27.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v27 => /ROOT/projects/l2-resource-option-parent/sdks/simple-27.0.0
