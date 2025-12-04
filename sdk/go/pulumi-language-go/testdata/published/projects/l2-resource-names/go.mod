module l2-resource-names

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-names/sdk/go/v6 v6.0.0
)

replace example.com/pulumi-names/sdk/go/v6 => /ROOT/artifacts/example.com_pulumi-names_sdk_go_v6

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
