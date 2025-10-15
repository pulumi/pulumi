module l2-parameterized-resource-twice

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-byepackage/sdk/go/v2 v2.0.0
	example.com/pulumi-hipackage/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-byepackage/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-byepackage_sdk_go_v2

replace example.com/pulumi-hipackage/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-hipackage_sdk_go_v2

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
