module l2-resource-exists

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-exists/sdk/go/v60 v60.0.0
)

replace example.com/pulumi-exists/sdk/go/v60 => /ROOT/artifacts/example.com_pulumi-exists_sdk_go_v60

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
