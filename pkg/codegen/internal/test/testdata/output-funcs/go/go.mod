module codegentest

go 1.16

require (
	github.com/pulumi/pulumi/sdk/v3 v3.2.1
	github.com/stretchr/testify v1.6.1
)

replace github.com/pulumi/pulumi/sdk/v3 => ../../../../../../../sdk
