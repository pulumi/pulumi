module l2-namespaced-provider

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component/sdk/go/v13 v13.3.7
	github.com/a-namespace/pulumi-namespaced/sdk/go/v16 v16.0.0
)

replace example.com/pulumi-component/sdk/go/v13 => /ROOT/projects/l2-namespaced-provider/sdks/component-13.3.7

replace github.com/a-namespace/pulumi-namespaced/sdk/go/v16 => /ROOT/projects/l2-namespaced-provider/sdks/namespaced-16.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
