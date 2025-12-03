module l2-ref-ref

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-ref-ref/sdk/go/v12 v12.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-ref-ref/sdk/go/v12 => /ROOT/artifacts/example.com_pulumi-ref-ref_sdk_go_v12
