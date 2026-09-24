module l2-failed-create-continue-on-error-read

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-fail_on_create/sdk/go/v4 v4.0.0
	example.com/pulumi-read/sdk/go/v39 v39.0.0
)

replace example.com/pulumi-fail_on_create/sdk/go/v4 => /ROOT/projects/l2-failed-create-continue-on-error-read/sdks/fail_on_create-4.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-read/sdk/go/v39 => /ROOT/projects/l2-failed-create-continue-on-error-read/sdks/read-39.0.0
