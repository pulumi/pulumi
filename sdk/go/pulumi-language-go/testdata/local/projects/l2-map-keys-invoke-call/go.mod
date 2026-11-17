module l2-map-keys-invoke-call

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component/sdk/go/v13 v13.3.7
	example.com/pulumi-simple-invoke/sdk/go/v10 v10.0.0
)

replace example.com/pulumi-component/sdk/go/v13 => /ROOT/projects/l2-map-keys-invoke-call/sdks/component-13.3.7

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple-invoke/sdk/go/v10 => /ROOT/projects/l2-map-keys-invoke-call/sdks/simple-invoke-10.0.0
