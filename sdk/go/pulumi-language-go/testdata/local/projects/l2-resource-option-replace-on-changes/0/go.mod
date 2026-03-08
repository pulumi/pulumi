module l2-resource-option-replace-on-changes

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-replaceonchanges/sdk/go/v25 v25.0.0
)

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-replaceonchanges/sdk/go/v25 => /ROOT/projects/l2-resource-option-replace-on-changes/0/sdks/replaceonchanges-25.0.0
