module l2-namespaced-provider

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	github.com/a-namespace/pulumi-namespaced/sdk/go/v16 v16.0.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace github.com/a-namespace/pulumi-namespaced/sdk/go/v16 => /ROOT/artifacts/github.com_a-namespace_pulumi-namespaced_sdk_go_v16

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-simple_sdk_go_v2
