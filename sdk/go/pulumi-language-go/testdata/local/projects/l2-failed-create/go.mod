module l2-failed-create

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-fail_on_create/sdk/go/v4 v4.0.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-fail_on_create/sdk/go/v4 => /ROOT/projects/l2-failed-create/sdks/fail_on_create-4.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/projects/l2-failed-create/sdks/simple-2.0.0
