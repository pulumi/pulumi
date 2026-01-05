module l2-parameterized-resource-twice

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-byepackage/sdk/go/v2 v2.0.0
	example.com/pulumi-hipackage/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-byepackage/sdk/go/v2 => /ROOT/projects/l2-parameterized-resource-twice/sdks/byepackage-2.0.0

replace example.com/pulumi-hipackage/sdk/go/v2 => /ROOT/projects/l2-parameterized-resource-twice/sdks/hipackage-2.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
