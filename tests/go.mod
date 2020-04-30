module github.com/pulumi/pulumi/tests

go 1.13

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.5.0
	github.com/pulumi/pulumi/pkg/v2 => ../pkg
	github.com/pulumi/pulumi/sdk/v2 => ../sdk
	gocloud.dev => github.com/pulumi/go-cloud v0.18.1-0.20191119155701-6a8381d0793f
)

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v2 v2.0.0
	github.com/pulumi/pulumi/sdk/v2 v2.0.0
	github.com/stretchr/testify v1.5.1
)
