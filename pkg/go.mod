module github.com/pulumi/pulumi/pkg/v2

go 1.13

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.5.0
	github.com/pulumi/pulumi/pkg => ../pkg
	github.com/pulumi/pulumi/sdk => ../sdk
	gocloud.dev => github.com/pulumi/go-cloud v0.18.1-0.20191119155701-6a8381d0793f
)

require (
	cloud.google.com/go/logging v1.0.0
	cloud.google.com/go/storage v1.6.0
	github.com/Azure/go-autorest/autorest v0.10.0 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.2 // indirect
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/aws/aws-sdk-go v1.29.27
	github.com/blang/semver v3.5.1+incompatible
	github.com/djherbis/times v1.2.0
	github.com/docker/docker v0.0.0-20170504205632-89658bed64c2
	github.com/dustin/go-humanize v1.0.0
	github.com/golang/protobuf v1.3.5
	github.com/google/go-querystring v1.0.0
	github.com/gorilla/mux v1.7.4
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/ijc/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/copystructure v1.0.0
	github.com/mxschmitt/golang-combinations v1.0.0
	github.com/nbutton23/zxcvbn-go v0.0.0-20180912185939-ae427f1e4c1d
	github.com/opentracing/basictracer-go v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg v0.0.0-00010101000000-000000000000
	github.com/pulumi/pulumi/sdk v1.13.1
	github.com/rjeczalik/notify v0.9.2
	github.com/satori/go.uuid v1.2.0
	github.com/sergi/go-diff v1.1.0
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.5.1
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/zclconf/go-cty v1.3.1
	gocloud.dev v0.19.0
	gocloud.dev/secrets/hashivault v0.19.0
	golang.org/x/crypto v0.0.0-20200317142112-1b76d66859c6
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	google.golang.org/api v0.20.0
	google.golang.org/genproto v0.0.0-20200318110522-7735f76e9fa5
	google.golang.org/grpc v1.28.0
	gopkg.in/AlecAivazis/survey.v1 v1.8.8
	gopkg.in/src-d/go-git.v4 v4.13.1
	sourcegraph.com/sourcegraph/appdash v0.0.0-20190731080439-ebfcffb1b5c0
	sourcegraph.com/sourcegraph/appdash-data v0.0.0-20151005221446-73f23eafcf67 // indirect
)
