module github.com/pulumi/pulumi/tests

go 1.16

replace (
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.5.0
	github.com/pulumi/pulumi/pkg/v3 => ../pkg
	github.com/pulumi/pulumi/sdk/v3 => ../sdk
)

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v3 v3.3.0
	github.com/pulumi/pulumi/sdk/v3 v3.3.1
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0 // indirect
	google.golang.org/grpc v1.37.0
	sourcegraph.com/sourcegraph/appdash v0.0.0-20190731080439-ebfcffb1b5c0 // indirect
)
