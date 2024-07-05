package main

import (
	"github.com/pulumi/pulumi-unknown/sdk/go/unknown"
	"github.com/pulumi/pulumi-unknown/sdk/go/unknown/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := unknown.NewProvider(ctx, "provider", nil)
		if err != nil {
			return err
		}
		main, err := unknown.NewMain(ctx, "main", &unknown.MainArgs{
			First: "hello",
			Second: map[string]interface{}{
				"foo": "bar",
			},
		})
		if err != nil {
			return err
		}
		var fromModule []*eks.Example
		for index := 0; index < 10; index++ {
			key0 := index
			_ := index
			__res, err := eks.NewExample(ctx, fmt.Sprintf("fromModule-%v", key0), &eks.ExampleArgs{
				AssociatedMain: main.Id,
			})
			if err != nil {
				return err
			}
			fromModule = append(fromModule, __res)
		}
		ctx.Export("mainId", main.Id)
		ctx.Export("values", fromModule.Values.First)
		return nil
	})
}
