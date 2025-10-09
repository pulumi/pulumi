module l2-invoke-secrets

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
	example.com/pulumi-simple-invoke/sdk/go/v10 v10.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/projects/l2-invoke-secrets/sdks/simple-2.0.0

replace example.com/pulumi-simple-invoke/sdk/go/v10 => /ROOT/projects/l2-invoke-secrets/sdks/simple-invoke-10.0.0
