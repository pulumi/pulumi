package main

import (
	"example.com/pulumi-splatoptional/sdk/go/splatoptional"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		source, err := splatoptional.NewContainer(ctx, "source", &splatoptional.ContainerArgs{
			Inputs: pulumi.StringArray{
				pulumi.String("a"),
				pulumi.String("b"),
			},
		})
		if err != nil {
			return err
		}
		_, err = splatoptional.NewContainer(ctx, "sink", &splatoptional.ContainerArgs{
			Inputs: source.Details.ApplyT(func(details []splatoptional.Detail) ([]string, error) {
				var splat0 []string
				for _, val0 := range details {
					splat0 = append(splat0, *val0.Value)
				}
				return splat0, nil
			}).(pulumi.StringArrayOutput),
		})
		if err != nil {
			return err
		}
		ctx.Export("values", source.Details.ApplyT(func(details []splatoptional.Detail) ([]string, error) {
			var splat1 []string
			for _, val0 := range details {
				splat1 = append(splat1, *val0.Value)
			}
			return splat1, nil
		}).(pulumi.StringArrayOutput))
		return nil
	})
}
