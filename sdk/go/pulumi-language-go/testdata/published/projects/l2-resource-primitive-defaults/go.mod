module l2-resource-primitive-defaults

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-primitive-defaults/sdk/go/v8 v8.0.0
)

replace example.com/pulumi-primitive-defaults/sdk/go/v8 => /ROOT/artifacts/example.com_pulumi-primitive-defaults_sdk_go_v8

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace github.com/pulumi/pulumi/shared/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_shared_v3
