module l2-explicit-parameterized-provider

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-goodbye/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-goodbye/sdk/go/v2 => /tmp/TestLanguagelocal=true1926289778/001/projects/l2-explicit-parameterized-provider/sdks/goodbye-2.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
