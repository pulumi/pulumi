module github.com/pulumi/pulumi/sdk/go/generate/v3

go 1.20

replace (
	github.com/pulumi/pulumi/pkg/v3 => ../../../pkg
	github.com/pulumi/pulumi/sdk/v3 => ../../
)

require (
	github.com/pulumi/pulumi/sdk/v3 v3.55.0
	golang.org/x/text v0.14.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230706204954-ccb25ca9f130 // indirect
	google.golang.org/grpc v1.57.1 // indirect
)
