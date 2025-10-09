module l2-resource-secret

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-secret/sdk/go/v14 v14.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-secret/sdk/go/v14 => /tmp/TestLanguagelocal=true1926289778/001/projects/l2-resource-secret/sdks/secret-14.0.0
