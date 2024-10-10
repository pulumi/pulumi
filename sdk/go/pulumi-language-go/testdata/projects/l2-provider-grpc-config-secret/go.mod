module l2-provider-grpc-config-secret

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-testconfigprovider/sdk/go v1.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-testconfigprovider/sdk/go => /ROOT/artifacts/example.com_pulumi-testconfigprovider_sdk_go
