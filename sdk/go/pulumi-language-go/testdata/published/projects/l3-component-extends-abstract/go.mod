module l3-component-extends-abstract

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-inheritabstract/sdk/go v1.0.0
)

replace example.com/pulumi-inheritabstract/sdk/go => /ROOT/artifacts/example.com_pulumi-inheritabstract_sdk_go

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
