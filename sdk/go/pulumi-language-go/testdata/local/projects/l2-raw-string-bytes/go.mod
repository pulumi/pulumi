module l2-raw-string-bytes

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-bytesink/sdk/go/v47 v47.0.0
	example.com/pulumi-bytesource/sdk/go/v48 v48.0.0
)

replace example.com/pulumi-bytesink/sdk/go/v47 => /ROOT/projects/l2-raw-string-bytes/sdks/bytesink-47.0.0

replace example.com/pulumi-bytesource/sdk/go/v48 => /ROOT/projects/l2-raw-string-bytes/sdks/bytesource-48.0.0

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
