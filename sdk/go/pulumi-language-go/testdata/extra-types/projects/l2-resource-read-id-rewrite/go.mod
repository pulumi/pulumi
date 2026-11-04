module l2-resource-read-id-rewrite

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-read-id-rewrite/sdk/go/v58 v58.0.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-read-id-rewrite/sdk/go/v58 => /ROOT/artifacts/example.com_pulumi-read-id-rewrite_sdk_go_v58

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-simple_sdk_go_v2
