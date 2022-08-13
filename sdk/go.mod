module github.com/mariospas/pulumi/sdk/v3

go 1.17

replace golang.org/x/text => golang.org/x/text v0.3.6

require (
	github.com/Microsoft/go-winio v0.4.14
	github.com/blang/semver v3.5.1+incompatible
	github.com/cheggaaa/pb v1.0.18
	github.com/djherbis/times v1.2.0
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.2
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/go-multierror v1.0.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/nxadm/tail v1.4.8
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/rivo/uniseg v0.2.0
	github.com/rogpeppe/go-internal v1.8.1
	github.com/sabhiram/go-gitignore v0.0.0-20180611051255-d3107576ba94
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/texttheater/golang-levenshtein v0.0.0-20191208221605-eb6844b05fc6
	github.com/tweekmonster/luser v0.0.0-20161003172636-3fa38070dbd7
	github.com/uber/jaeger-client-go v2.22.1+incompatible
	golang.org/x/crypto v0.0.0-20200317142112-1b76d66859c6
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	golang.org/x/sys v0.0.0-20210817190340-bfb29a6856f2
	google.golang.org/grpc v1.29.1
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.8
	pgregory.net/rapid v0.4.7
	sourcegraph.com/sourcegraph/appdash v0.0.0-20190731080439-ebfcffb1b5c0
)

require (
	github.com/hashicorp/go-version v1.4.0
	github.com/pulumi/pulumi/sdk/v3 v3.26.1
)
