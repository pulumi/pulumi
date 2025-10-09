module l2-resource-alpha

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-alpha/sdk/go/v3 v3.0.0-alpha.1.internal
)

replace example.com/pulumi-alpha/sdk/go/v3 => /tmp/TestLanguagelocal=true1926289778/001/projects/l2-resource-alpha/sdks/alpha-3.0.0-alpha.1.internal+exp.sha.12345678

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
