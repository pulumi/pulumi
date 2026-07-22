module l2-allowed-package-name

go 1.25

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-extra-package-names/sdk/go/v47 v47.0.0
)

replace example.com/pulumi-extra-package-names/sdk/go/v47 => /ROOT/artifacts/example.com_pulumi-extra-package-names_sdk_go_v47

replace github.com/pulumi/pulumi/sdk/v3 => /ROOT/artifacts/github.com_pulumi_pulumi_sdk_v3
