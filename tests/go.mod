module github.com/pulumi/pulumi/tests

go 1.13

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.5.0
	github.com/pulumi/pulumi/pkg => ../pkg
	github.com/pulumi/pulumi/sdk => ../sdk
	gocloud.dev => github.com/pulumi/go-cloud v0.18.1-0.20191119155701-6a8381d0793f
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Sirupsen/logrus v0.0.0-00010101000000-000000000000 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg v0.0.0-00010101000000-000000000000
	github.com/pulumi/pulumi/sdk v1.13.1
	github.com/stretchr/testify v1.5.1
	gopkg.in/cheggaaa/pb.v1 v1.0.28 // indirect
)
