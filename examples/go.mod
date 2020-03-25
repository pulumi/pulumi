module github.com/pulumi/pulumi/examples

go 1.13

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
	github.com/pulumi/pulumi/pkg => ../pkg
	github.com/pulumi/pulumi/sdk => ../sdk
	gocloud.dev => github.com/pulumi/go-cloud v0.18.1-0.20191119155701-6a8381d0793f
)

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg v0.0.0-00010101000000-000000000000
	github.com/pulumi/pulumi/sdk v0.0.0-20200321193742-f095e64d0f8e
	github.com/stretchr/testify v1.5.1
)
