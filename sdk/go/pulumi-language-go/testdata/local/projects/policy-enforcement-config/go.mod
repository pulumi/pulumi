module policy-enforcement-config

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple/sdk/go/v2 => /tmp/TestLanguagelocal=true1926289778/001/projects/policy-enforcement-config/sdks/simple-2.0.0
