module github.com/pulumi/pulumi/developer-docs

go 1.22

toolchain go1.22.1

replace github.com/pulumi/pulumi/sdk/v3 => ../sdk

require (
	github.com/pulumi/pulumi/sdk/v3 v3.73.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.0.0
)
