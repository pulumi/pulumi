module l2-resource-read

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-read/sdk/go/v39 v39.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-read/sdk/go/v39 => /ROOT/artifacts/example.com_pulumi-read_sdk_go_v39
