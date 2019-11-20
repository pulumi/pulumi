module github.com/pulumi/pulumi

go 1.12

require (
	cloud.google.com/go v0.39.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Microsoft/go-winio v0.4.14
	github.com/Sirupsen/logrus v1.0.5 // indirect
	github.com/aws/aws-sdk-go v1.19.45
	github.com/blang/semver v3.5.1+incompatible
	github.com/cheggaaa/pb v1.0.18
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/cpuguy83/go-md2man v1.0.8 // indirect
	github.com/djherbis/times v1.0.1
	github.com/docker/docker v0.0.0-20170504205632-89658bed64c2
	github.com/dustin/go-humanize v1.0.0
	github.com/gofrs/flock v0.7.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.1
	github.com/google/go-querystring v1.0.0
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20171105060200-01f8541d5372
	github.com/hashicorp/go-multierror v1.0.0
	github.com/ijc/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mattn/go-runewidth v0.0.3 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/mxschmitt/golang-combinations v1.0.0
	github.com/nbutton23/zxcvbn-go v0.0.0-20171102151520-eafdab6b0663
	github.com/onsi/ginkgo v1.7.0 // indirect
	github.com/onsi/gomega v1.4.3 // indirect
	github.com/opentracing/basictracer-go v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.0.2
	github.com/pkg/errors v0.8.1
	github.com/rjeczalik/notify v0.9.2
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sergi/go-diff v1.0.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749 // indirect
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd // indirect
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/spf13/cast v1.2.0
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/texttheater/golang-levenshtein v0.0.0-20180516184445-d188e65d659e
	github.com/uber/jaeger-client-go v2.15.0+incompatible
	github.com/uber/jaeger-lib v1.5.0 // indirect
	gocloud.dev v0.18.0
	gocloud.dev/secrets/hashivault v0.18.0
	golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5
	golang.org/x/net v0.0.0-20190619014844-b5b0513f8c1b
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190620070143-6f217b454f45
	google.golang.org/api v0.6.0
	google.golang.org/genproto v0.0.0-20190620144150-6af8c5fc6601
	google.golang.org/grpc v1.21.1
	gopkg.in/AlecAivazis/survey.v1 v1.4.1
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/src-d/go-git-fixtures.v3 v3.4.0 // indirect
	gopkg.in/src-d/go-git.v4 v4.8.1
	gopkg.in/yaml.v2 v2.2.2
	sourcegraph.com/sourcegraph/appdash v0.0.0-20190731080439-ebfcffb1b5c0
	sourcegraph.com/sourcegraph/appdash-data v0.0.0-20151005221446-73f23eafcf67 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
	gocloud.dev => github.com/pulumi/go-cloud v0.18.1-0.20191119155701-6a8381d0793f
)
