module l2-resource-option-replace-on-changes

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-component/sdk/go/v13 v13.3.7
	example.com/pulumi-replaceonchanges/sdk/go/v25 v25.0.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-component/sdk/go/v13 => /ROOT/projects/l2-resource-option-replace-on-changes/1/sdks/component-13.3.7

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-replaceonchanges/sdk/go/v25 => /ROOT/projects/l2-resource-option-replace-on-changes/1/sdks/replaceonchanges-25.0.0

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/projects/l2-resource-option-replace-on-changes/1/sdks/simple-2.0.0
