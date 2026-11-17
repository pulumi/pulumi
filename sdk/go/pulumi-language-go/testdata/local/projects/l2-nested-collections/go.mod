module l2-nested-collections

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-nestedcollections/sdk/go/v50 v50.0.0
)

replace example.com/pulumi-nestedcollections/sdk/go/v50 => /ROOT/projects/l2-nested-collections/sdks/nestedcollections-50.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
