module github.com/pulumi/pulumi/pkg/v2/codegen/internal/test/testdata/external-resource-schema/go

go 1.14

// TODO need to bump these to latest after XX merges
require (
	github.com/pulumi/pulumi-aws/sdk/v3 v3.19.2
	github.com/pulumi/pulumi-kubernetes/sdk/v2 v2.7.2
	github.com/pulumi/pulumi-random/sdk/v2 v2.4.1
	github.com/pulumi/pulumi/sdk/v2 v2.14.0
	github.com/stretchr/testify v1.5.1
)

replace (
	github.com/pulumi/pulumi-aws/sdk/v3 => /Users/vivekl/code/pulumi-aws/sdk
	github.com/pulumi/pulumi-kubernetes/sdk/v2 => /users/vivekl/code/pulumi-kubernetes/sdk
	github.com/pulumi/pulumi-random/sdk/v2 => /Users/vivekl/code/pulumi-random/sdk
)
