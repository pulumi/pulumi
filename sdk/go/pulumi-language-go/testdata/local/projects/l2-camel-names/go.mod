module l2-camel-names

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-camelNames/sdk/go/v19 v19.0.0
)

replace example.com/pulumi-camelNames/sdk/go/v19 => /ROOT/projects/l2-camel-names/sdks/camelNames-19.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
