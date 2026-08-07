module l2-discriminated-union-internal

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-discriminated-union-internal/sdk/go/v50 v50.0.0
)

replace example.com/pulumi-discriminated-union-internal/sdk/go/v50 => /ROOT/artifacts/example.com_pulumi-discriminated-union-internal_sdk_go_v50

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
