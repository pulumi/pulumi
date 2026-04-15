module github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3

go 1.25.8

replace (
	github.com/atotto/clipboard => github.com/tgummerer/clipboard v0.0.0-20241001131231-d02d263e614e
	github.com/pulumi/pulumi/pkg/v3 => ../../../../pkg
	github.com/pulumi/pulumi/sdk/v3 => ../../..
)

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/go-multierror v1.1.1
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pulumi/pulumi/pkg/v3 v3.243.0
	github.com/pulumi/pulumi/sdk/v3 v3.246.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
	golang.org/x/sync v0.21.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bazelbuild/buildtools v0.0.0-20260211083412-859bfffeef82 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/bubbles v1.0.0 // indirect
	github.com/charmbracelet/bubbletea v1.3.10 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.7 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/djherbis/times v1.5.0 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/git-pkgs/manifests v0.4.1 // indirect
	github.com/git-pkgs/packageurl-go v0.3.1 // indirect
	github.com/git-pkgs/purl v0.1.10 // indirect
	github.com/git-pkgs/vers v0.2.4 // indirect
	github.com/go-git/gcfg/v2 v2.0.2 // indirect
	github.com/go-git/go-billy/v6 v6.0.0-alpha.1 // indirect
	github.com/go-git/go-git/v6 v6.0.0-alpha.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/hcl/v2 v2.24.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/opentracing/basictracer-go v1.1.0 // indirect
	github.com/pgavlin/fx v0.1.6 // indirect
	github.com/pgavlin/fx/v2 v2.0.12 // indirect
	github.com/pgavlin/goldmark v1.1.33-0.20200616210433-b5eb04559386 // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/term v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/pulumi/appdash v0.0.0-20231130102222-75f619a67231 // indirect
	github.com/pulumi/esc v0.25.0 // indirect
	github.com/pulumi/inflector v0.1.1 // indirect
	github.com/quasilyte/go-ruleguard/dsl v0.3.23 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.0.0 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.3.5 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/texttheater/golang-levenshtein v1.0.1 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/zclconf/go-cty v1.16.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.60.0 // indirect
	go.opentelemetry.io/collector/pdata v1.60.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelslog v0.18.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/log v0.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.19.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/term v0.44.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260610212136-7ab31c22f7ad // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260610212136-7ab31c22f7ad // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/frand v1.5.1 // indirect
)

tool github.com/quasilyte/go-ruleguard/dsl
