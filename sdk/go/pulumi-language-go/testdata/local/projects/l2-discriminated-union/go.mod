module l2-discriminated-union

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-discriminated-union/sdk/go/v30 v30.0.0
)

replace example.com/pulumi-discriminated-union/sdk/go/v30 => /ROOT/projects/l2-discriminated-union/sdks/discriminated-union-30.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
