module l2-resource-asset-archive

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-asset-archive/sdk/go/v5 v5.0.0
)

replace example.com/pulumi-asset-archive/sdk/go/v5 => /ROOT/projects/l2-resource-asset-archive/sdks/asset-archive-5.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
