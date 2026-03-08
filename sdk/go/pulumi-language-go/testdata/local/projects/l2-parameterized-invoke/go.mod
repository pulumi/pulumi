module l2-parameterized-invoke

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-subpackage/sdk/go/v2 v2.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-subpackage/sdk/go/v2 => /ROOT/projects/l2-parameterized-invoke/sdks/subpackage-2.0.0
