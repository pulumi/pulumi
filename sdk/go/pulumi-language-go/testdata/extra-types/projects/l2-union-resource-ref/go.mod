module l2-union-resource-ref

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-unionref/sdk/go/v63 v63.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-unionref/sdk/go/v63 => /ROOT/artifacts/example.com_pulumi-unionref_sdk_go_v63
