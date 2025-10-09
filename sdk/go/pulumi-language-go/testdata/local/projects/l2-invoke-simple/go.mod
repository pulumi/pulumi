module l2-invoke-simple

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-simple-invoke/sdk/go/v10 v10.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-simple-invoke/sdk/go/v10 => /tmp/TestLanguagelocal=true1926289778/001/projects/l2-invoke-simple/sdks/simple-invoke-10.0.0
