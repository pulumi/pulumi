module l2-module-format-enum

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-moduleformatenum/sdk/go/v64 v64.0.0
)

replace example.com/pulumi-moduleformatenum/sdk/go/v64 => /ROOT/projects/l2-module-format-enum/sdks/moduleformatenum-64.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
