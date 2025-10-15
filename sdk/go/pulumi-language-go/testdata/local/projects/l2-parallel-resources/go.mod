module l2-parallel-resources

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-sync/sdk/go/v3 v3.0.0-alpha.1.internal
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-sync/sdk/go/v3 => /ROOT/projects/l2-parallel-resources/sdks/sync-3.0.0-alpha.1.internal+exp.sha.2143768
