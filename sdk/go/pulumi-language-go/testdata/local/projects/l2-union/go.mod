module l2-union

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-union/sdk/go/v18 v18.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-union/sdk/go/v18 => /tmp/TestLanguagelocal=true1926289778/001/projects/l2-union/sdks/union-18.0.0
