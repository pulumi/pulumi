module l2-component-call-plain

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-configurer/sdk/go/v38 v38.0.0
)

replace example.com/pulumi-configurer/sdk/go/v38 => /ROOT/artifacts/example.com_pulumi-configurer_sdk_go_v38

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
