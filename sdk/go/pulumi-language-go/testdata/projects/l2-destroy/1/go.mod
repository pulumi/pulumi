module l2-destroy

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	github.com/pulumi/pulumi-simple/sdk/v2 v2.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace github.com/pulumi/pulumi-simple/sdk/v2 => /ROOT/artifacts/github.com_pulumi_pulumi-simple_sdk_v2
