module github.com/pulumi/pulumi/docs

go 1.26.6

replace github.com/pulumi/pulumi/sdk/v3 => ../../sdk

require (
	github.com/pulumi/pulumi/shared/v3 v3.269.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.0.0
)

replace github.com/pulumi/pulumi/shared/v3 => ../../shared
