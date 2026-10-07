module l2-discriminated-union-many

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-discriminated-union-many/sdk/go/v49 v49.0.0
)

replace example.com/pulumi-discriminated-union-many/sdk/go/v49 => /ROOT/projects/l2-discriminated-union-many/sdks/discriminated-union-many-49.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
