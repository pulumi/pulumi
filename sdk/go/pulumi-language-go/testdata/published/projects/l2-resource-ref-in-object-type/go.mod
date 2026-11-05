module l2-resource-ref-in-object-type

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-objectref/sdk/go/v62 v62.0.0
)

replace example.com/pulumi-objectref/sdk/go/v62 => /ROOT/artifacts/example.com_pulumi-objectref_sdk_go_v62

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
