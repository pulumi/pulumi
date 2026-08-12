module l2-name-conflicts

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-module-format/sdk/go/v29 v29.0.0
	example.com/pulumi-names/sdk/go/v6 v6.0.0
)

replace example.com/pulumi-module-format/sdk/go/v29 => /ROOT/projects/l2-name-conflicts/sdks/module-format-29.0.0

replace example.com/pulumi-names/sdk/go/v6 => /ROOT/projects/l2-name-conflicts/sdks/names-6.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
