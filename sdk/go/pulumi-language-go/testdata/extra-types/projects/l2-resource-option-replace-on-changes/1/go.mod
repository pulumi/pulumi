module l2-resource-option-replace-on-changes

go 1.23

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-conformance-component/sdk/go/v22 v22.0.0
	example.com/pulumi-replaceonchanges/sdk/go/v25 v25.0.0
	example.com/pulumi-simple/sdk/go/v2 v2.0.0
)

replace example.com/pulumi-conformance-component/sdk/go/v22 => /ROOT/artifacts/example.com_pulumi-conformance-component_sdk_go_v22

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3

replace example.com/pulumi-replaceonchanges/sdk/go/v25 => /ROOT/artifacts/example.com_pulumi-replaceonchanges_sdk_go_v25

replace example.com/pulumi-simple/sdk/go/v2 => /ROOT/artifacts/example.com_pulumi-simple_sdk_go_v2
