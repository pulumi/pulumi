module l2-large-string

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-large/sdk/go/v4 v4.3.2
)

replace example.com/pulumi-large/sdk/go/v4 => /ROOT/artifacts/example.com_pulumi-large_sdk_go_v4

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
