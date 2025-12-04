package main

import (
	"example.com/pulumi-simple-invoke-with-scalar-return/sdk/go/v17/simpleinvokewithscalarreturn"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("scalar", simpleinvokewithscalarreturn.MyInvokeScalarOutput(ctx, simpleinvokewithscalarreturn.MyInvokeScalarOutputArgs{
			Value: pulumi.String("goodbye"),
		}, nil))
		return nil
	})
}
