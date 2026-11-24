module l2-failed-read-continue-on-error

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-read/sdk/go/v39 v39.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-read/sdk/go/v39 => /ROOT/projects/l2-failed-read-continue-on-error/sdks/read-39.0.0
