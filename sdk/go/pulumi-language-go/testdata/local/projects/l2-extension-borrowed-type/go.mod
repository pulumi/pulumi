module l2-extension-borrowed-type

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-borrowbase/sdk/go/v46 v46.0.0
	example.com/pulumi-myborrow/sdk/go/v3 v3.0.0
)

replace example.com/pulumi-borrowbase/sdk/go/v46 => /ROOT/projects/l2-extension-borrowed-type/sdks/borrowbase-46.0.0

replace example.com/pulumi-myborrow/sdk/go/v3 => /ROOT/projects/l2-extension-borrowed-type/sdks/myborrow-3.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
