module l2-discriminated-union

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-discriminated-union/sdk/go/v31 v31.0.0
)

replace example.com/pulumi-discriminated-union/sdk/go/v31 => /ROOT/artifacts/example.com_pulumi-discriminated-union_sdk_go_v31

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
