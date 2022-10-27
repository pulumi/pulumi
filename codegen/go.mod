module github.com/pulumi/pulumi/codegen/v3

go 1.17

replace github.com/pulumi/pulumi/sdk/v3 => ../sdk

// Working around https://github.com/sergi/go-diff/issues/123
replace github.com/sergi/go-diff => github.com/sergi/go-diff v1.1.0

replace github.com/xanzy/ssh-agent => github.com/pulumi/ssh-agent v0.5.1

require (
	cloud.google.com/go/logging v1.0.0 // indirect
	cloud.google.com/go/storage v1.24.0 // indirect
	github.com/aws/aws-sdk-go v1.44.68 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/djherbis/times v1.5.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl/v2 v2.14.0
	github.com/iancoleman/strcase v0.2.0
	github.com/ijc/Gotty v0.0.0-20170406111628-a8b993ba6abd // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pgavlin/goldmark v1.1.33-0.20200616210433-b5eb04559386
	github.com/pulumi/pulumi/sdk/v3 v3.44.2
	github.com/santhosh-tekuri/jsonschema/v5 v5.0.0
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/spf13/cobra v1.5.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.8.0
	github.com/tweekmonster/luser v0.0.0-20161003172636-3fa38070dbd7 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/zclconf/go-cty v1.10.0
	gocloud.dev v0.27.0 // indirect
	gocloud.dev/secrets/hashivault v0.24.0 // indirect
	golang.org/x/crypto v0.0.0-20220824171710-5757bc0c5503 // indirect
	golang.org/x/net v0.0.0-20220802222814-0bcc04d9c69b // indirect
	golang.org/x/oauth2 v0.0.0-20220722155238-128564f6959c // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	google.golang.org/api v0.91.0 // indirect
	google.golang.org/genproto v0.0.0-20220802133213-ce4fa296bf78 // indirect
	google.golang.org/grpc v1.49.0 // indirect
	gopkg.in/AlecAivazis/survey.v1 v1.8.9-0.20200217094205-6773bdf39b7f // indirect
	gopkg.in/yaml.v3 v3.0.1
	pgregory.net/rapid v0.4.7
	sourcegraph.com/sourcegraph/appdash v0.0.0-20211028080628-e2786a622600 // indirect
)

require (
	github.com/edsrzf/mmap-go v1.1.0
	github.com/natefinch/atomic v1.0.1
	github.com/pulumi/pulumi-java/pkg v0.6.0
	github.com/pulumi/pulumi-yaml v0.5.10
	github.com/pulumi/pulumi/pkg/v3 v3.44.2
	github.com/segmentio/encoding v0.3.5
)

require (
	cloud.google.com/go v0.103.0 // indirect
	cloud.google.com/go/compute v1.7.0 // indirect
	cloud.google.com/go/iam v0.3.0 // indirect
	cloud.google.com/go/kms v1.4.0 // indirect
	github.com/Azure/azure-sdk-for-go v66.0.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.1.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v0.4.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v0.4.0 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210428141323-04723f9f07d7 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.16.8 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.15.15 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.9 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.18.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.27.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.10 // indirect
	github.com/aws/smithy-go v1.12.0 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/ettle/strcase v0.1.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/google/wire v0.5.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.4.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/vault/api v1.1.1 // indirect
	github.com/hashicorp/vault/sdk v0.2.1 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.0 // indirect
	github.com/opentracing/basictracer-go v1.1.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/browser v0.0.0-20210115035449-ce105d075bb4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/term v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/texttheater/golang-levenshtein v1.0.1 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/xanzy/ssh-agent v0.3.2 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20220823224334-20c2bfdbfe24 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	lukechampine.com/frand v1.4.2 // indirect
)
