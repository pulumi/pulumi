module l2-discriminated-union-marked-key

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-discriminated-union-marked-key/sdk/go/v53 v53.0.0
)

replace example.com/pulumi-discriminated-union-marked-key/sdk/go/v53 => /ROOT/projects/l2-discriminated-union-marked-key/sdks/discriminated-union-marked-key-53.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
