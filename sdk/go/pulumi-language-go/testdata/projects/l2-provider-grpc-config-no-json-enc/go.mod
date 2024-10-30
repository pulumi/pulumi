module l2-provider-grpc-config-no-json-enc

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-config-grpc-no-jsonenc/sdk/go v1.0.0
)

replace example.com/pulumi-config-grpc-no-jsonenc/sdk/go => /ROOT/artifacts/example.com_pulumi-config-grpc-no-jsonenc_sdk_go

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
