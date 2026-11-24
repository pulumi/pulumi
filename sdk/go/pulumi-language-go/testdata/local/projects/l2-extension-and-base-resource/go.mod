module l2-extension-and-base-resource

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-extbase/sdk/go/v45 v45.0.0
	example.com/pulumi-myext/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-extbase/sdk/go/v45 => /ROOT/projects/l2-extension-and-base-resource/sdks/extbase-45.0.0

replace example.com/pulumi-myext/sdk/go/v2 => /ROOT/projects/l2-extension-and-base-resource/sdks/myext-2.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
