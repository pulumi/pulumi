package main

import (
	"example.com/pulumi-docs/sdk/go/v28/docs"
	"example.com/pulumi-enum/sdk/go/v30/enum"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := enum.NewRes(ctx, "enumRes", &enum.ResArgs{
			IntEnum:    enum.IntEnumIntOne,
			StringEnum: enum.StringEnumStringOne,
		})
		if err != nil {
			return err
		}
		_, err = docs.NewResource(ctx, "res", &docs.ResourceArgs{
			In: docs.FunOutput(ctx, docs.FunOutputArgs{
				In: pulumi.Bool(false),
			}, nil).ApplyT(func(invoke docs.FunResult) (bool, error) {
				return invoke.Out, nil
			}).(pulumi.BoolOutput),
			ExternalEnum: enum.StringEnumStringOne,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
