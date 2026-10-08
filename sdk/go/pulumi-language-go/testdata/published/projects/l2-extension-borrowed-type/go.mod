module l2-extension-borrowed-type

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-borrowbase/sdk/go/v46 v46.0.0
	example.com/pulumi-myborrow/sdk/go/v3 v3.0.0
)

replace example.com/pulumi-borrowbase/sdk/go/v46 => /ROOT/artifacts/example.com_pulumi-borrowbase_sdk_go_v46

replace example.com/pulumi-myborrow/sdk/go/v3 => /ROOT/artifacts/myborrow

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
