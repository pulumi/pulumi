module l2-component-call-simple-liftedreturn

go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
	example.com/pulumi-componentreturnscalar/sdk/go/v18 v18.0.0
)

replace example.com/pulumi-componentreturnscalar/sdk/go/v18 => ../../artifacts/example.com_pulumi-componentreturnscalar_sdk_go_v18

replace github.com/pulumi/pulumi/sdk/v3 => ../../artifacts/github.com_pulumi_pulumi_sdk_v3
