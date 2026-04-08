module l2-resource-optional

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-optionalprimitive/sdk/go/v34 v34.0.0
	example.com/pulumi-primitive/sdk/go/v7 v7.0.0
)

replace example.com/pulumi-optionalprimitive/sdk/go/v34 => /ROOT/artifacts/example.com_pulumi-optionalprimitive_sdk_go_v34

replace example.com/pulumi-primitive/sdk/go/v7 => /ROOT/artifacts/example.com_pulumi-primitive_sdk_go_v7

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
