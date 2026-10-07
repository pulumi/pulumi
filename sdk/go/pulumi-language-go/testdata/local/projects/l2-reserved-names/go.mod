module l2-reserved-names

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-reservednames/sdk/go/v51 v51.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-reservednames/sdk/go/v51 => /ROOT/projects/l2-reserved-names/sdks/reservednames-51.0.0
