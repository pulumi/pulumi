module l2-provider-call

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-call/sdk/go/v15 v15.7.9
)

replace example.com/pulumi-call/sdk/go/v15 => ../../artifacts/example.com_pulumi-call_sdk_go_v15

replace github.com/pulumi/pulumi/sdk/v3 => ../../artifacts/github.com_pulumi_pulumi_sdk_v3
