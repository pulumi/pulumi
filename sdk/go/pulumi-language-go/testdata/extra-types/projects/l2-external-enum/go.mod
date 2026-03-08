module l2-external-enum

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-enum/sdk/go/v30 v30.0.0
	example.com/pulumi-extenumref/sdk/go/v32 v32.0.0
)

replace example.com/pulumi-enum/sdk/go/v30 => /ROOT/artifacts/example.com_pulumi-enum_sdk_go_v30

replace example.com/pulumi-extenumref/sdk/go/v32 => /ROOT/artifacts/example.com_pulumi-extenumref_sdk_go_v32

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
