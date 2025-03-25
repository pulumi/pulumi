module l2-primitive-ref

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-primitive-ref/sdk/go/v11 v11.0.0
)

replace example.com/pulumi-primitive-ref/sdk/go/v11 => /ROOT/artifacts/example.com_pulumi-primitive-ref_sdk_go_v11

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
