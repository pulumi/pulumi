module l2-plain

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-plain/sdk/go/v13 v13.0.0
)

replace example.com/pulumi-plain/sdk/go/v13 => /ROOT/artifacts/example.com_pulumi-plain_sdk_go_v13

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
