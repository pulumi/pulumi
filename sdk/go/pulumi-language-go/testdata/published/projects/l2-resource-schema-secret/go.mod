module l2-resource-schema-secret

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-output/sdk/go/v23 v23.0.0
)

replace example.com/pulumi-output/sdk/go/v23 => /ROOT/artifacts/example.com_pulumi-output_sdk_go_v23

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
