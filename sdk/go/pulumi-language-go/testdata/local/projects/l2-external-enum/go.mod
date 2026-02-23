module l2-external-enum

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-enum/sdk/go/v30 v30.0.0
	example.com/pulumi-extenumref/sdk/go/v32 v32.0.0
)

replace example.com/pulumi-enum/sdk/go/v30 => /ROOT/projects/l2-external-enum/sdks/enum-30.0.0

replace example.com/pulumi-extenumref/sdk/go/v32 => /ROOT/projects/l2-external-enum/sdks/extenumref-32.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
