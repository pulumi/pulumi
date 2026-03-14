package main

import (
	"example.com/pulumi-nestedobject/sdk/go/nestedobject"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		source, err := nestedobject.NewContainer(ctx, "source", &nestedobject.ContainerArgs{
			Inputs: pulumi.StringArray{
				pulumi.String("a"),
				pulumi.String("b"),
			},
		})
		if err != nil {
			return err
		}
		_, err = nestedobject.NewContainer(ctx, "sink", &nestedobject.ContainerArgs{
			Inputs: source.Details.ApplyT(func(details []nestedobject.Detail) ([]string, error) {
				var splat0 []string
				for _, val0 := range details {
					splat0 = append(splat0, val0.Value)
				}
				return splat0, nil
			}).(pulumi.StringArrayOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
