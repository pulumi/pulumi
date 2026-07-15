module l2-resource-hook-on-error

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-flaky/sdk/go/v46 v46.0.0
)

replace example.com/pulumi-flaky/sdk/go/v46 => /ROOT/artifacts/example.com_pulumi-flaky_sdk_go_v46

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
