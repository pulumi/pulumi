module github.com/pulumi/pulumi/auto/v3

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/nxadm/tail v1.4.8
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v3 v3.3.1
	github.com/pulumi/pulumi/sdk/v3 v3.3.1
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.37.0
	gopkg.in/src-d/go-git.v4 v4.13.1
)

replace github.com/pulumi/pulumi/pkg/v3 => ../pkg
