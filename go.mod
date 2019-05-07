module github.com/pulumi/pulumi

go 1.12

require (
	cloud.google.com/go v0.37.2
	github.com/Nvveen/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/Sirupsen/logrus v1.0.5 // indirect
	github.com/aws/aws-sdk-go v1.16.23
	github.com/blang/semver v3.5.1+incompatible
	github.com/cheggaaa/pb v1.0.18
	github.com/cpuguy83/go-md2man v1.0.8 // indirect
	github.com/djherbis/times v1.0.1
	github.com/docker/docker v1.13.1
	github.com/dustin/go-humanize v1.0.0
	github.com/go-ini/ini v1.31.0 // indirect
	github.com/gofrs/flock v0.7.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.2.0
	github.com/google/go-querystring v1.0.0
	github.com/gorilla/mux v1.6.2
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20171105060200-01f8541d5372
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/nbutton23/zxcvbn-go v0.0.0-20171102151520-eafdab6b0663
	github.com/opentracing/opentracing-go v1.0.2
	github.com/pkg/errors v0.8.1
	github.com/reconquest/loreley v0.0.0-20160708080500-2ab6b7470a54
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sergi/go-diff v1.0.0
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a // indirect
	github.com/spf13/cast v1.2.0
	github.com/spf13/cobra v0.0.3
	github.com/stretchr/testify v1.2.2
	github.com/texttheater/golang-levenshtein v0.0.0-20180516184445-d188e65d659e
	github.com/uber/jaeger-client-go v2.15.0+incompatible
	go.opencensus.io v0.20.0 // indirect
	gocloud.dev v0.13.0
	golang.org/x/crypto v0.0.0-20190325154230-a5d413f7728c
	golang.org/x/net v0.0.0-20190328230028-74de082e2cca
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a // indirect
	golang.org/x/sync v0.0.0-20190227155943-e225da77a7e6
	google.golang.org/api v0.3.0
	google.golang.org/genproto v0.0.0-20190307195333-5fe7a883aa19
	google.golang.org/grpc v1.19.0
	gopkg.in/AlecAivazis/survey.v1 v1.4.1
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/ini.v1 v1.42.0 // indirect
	gopkg.in/src-d/go-git-fixtures.v3 v3.4.0 // indirect
	gopkg.in/src-d/go-git.v4 v4.8.1
	gopkg.in/yaml.v2 v2.2.2
)

replace (
	github.com/Nvveen/Gotty => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/golang/glog => github.com/pulumi/glog v0.0.0-20180820174630-7eaa6ffb71e4
)
