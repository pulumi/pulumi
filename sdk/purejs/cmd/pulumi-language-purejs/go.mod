module github.com/pulumi/pulumi/sdk/purejs/cmd/pulumi-language-purejs/v3

go 1.26.6

replace github.com/pulumi/pulumi/sdk/v3 => ../../..

require (
	github.com/dop251/goja v0.0.0-20260926152631-39ec2650adc9
	github.com/pulumi/pulumi/sdk/v3 v3.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.12.1
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/quasilyte/go-ruleguard/dsl v0.3.23 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
)

tool github.com/quasilyte/go-ruleguard/dsl
