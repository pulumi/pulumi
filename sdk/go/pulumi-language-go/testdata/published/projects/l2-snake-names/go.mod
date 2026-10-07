module l2-snake-names

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-snake_names/sdk/go/v33 v33.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-snake_names/sdk/go/v33 => /ROOT/artifacts/example.com_pulumi-snake_names_sdk_go_v33
