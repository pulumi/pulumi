module github.com/pulumi/pulumi/tests/integration/construct_component_configure_provider/testcomponent-go

go 1.25.8

replace (
	github.com/atotto/clipboard => github.com/tgummerer/clipboard v0.0.0-20241001131231-d02d263e614e
	github.com/pulumi/pulumi/pkg/v3 => ../../../../pkg
	github.com/pulumi/pulumi/sdk/v3 => ../../../../sdk
)
