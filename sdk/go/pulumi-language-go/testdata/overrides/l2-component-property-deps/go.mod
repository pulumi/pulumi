module l2-component-property-deps

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component-property-deps/sdk/go v1.33.7
)

replace example.com/pulumi-component-property-deps/sdk/go => ../../artifacts/example.com_pulumi-component-property-deps_sdk_go

replace github.com/pulumi/pulumi/sdk/v3 => ../../artifacts/github.com_pulumi_pulumi_sdk_v3
