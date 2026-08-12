module l2-map-keys

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-plain/sdk/go/v13 v13.0.0
	example.com/pulumi-primitive/sdk/go/v7 v7.0.0
	example.com/pulumi-primitive-ref/sdk/go/v11 v11.0.0
	example.com/pulumi-ref-ref/sdk/go/v12 v12.0.0
)

replace example.com/pulumi-plain/sdk/go/v13 => /ROOT/projects/l2-map-keys/sdks/plain-13.0.0

replace example.com/pulumi-primitive/sdk/go/v7 => /ROOT/projects/l2-map-keys/sdks/primitive-7.0.0

replace example.com/pulumi-primitive-ref/sdk/go/v11 => /ROOT/projects/l2-map-keys/sdks/primitive-ref-11.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-ref-ref/sdk/go/v12 => /ROOT/projects/l2-map-keys/sdks/ref-ref-12.0.0
