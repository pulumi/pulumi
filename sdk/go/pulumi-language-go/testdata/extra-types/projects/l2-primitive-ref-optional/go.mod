module l2-primitive-ref-optional

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-optional-primitive-ref/sdk/go/v40 v40.0.0
)

replace example.com/pulumi-optional-primitive-ref/sdk/go/v40 => /ROOT/artifacts/example.com_pulumi-optional-primitive-ref_sdk_go_v40

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace github.com/pulumi/pulumi/shared/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_shared_v3
