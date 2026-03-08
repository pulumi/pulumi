module l2-module-format

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-module-format/sdk/go/v29 v29.0.0
)

replace example.com/pulumi-module-format/sdk/go/v29 => /ROOT/projects/l2-module-format/sdks/module-format-29.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
