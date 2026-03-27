package main

import (
	"example.com/pulumi-snake_names/sdk/go/v33/snake_names/cool_module"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Resource inputs are correctly translated
		first, err := cool_module.NewSome_resource(ctx, "first", &cool_module.Some_resourceArgs{
			The_input: pulumi.Bool(true),
			Nested: &cool_module.Nested_inputArgs{
				Nested_value: pulumi.String("nested"),
			},
		})
		if err != nil {
			return err
		}
		// Datasource outputs are correctly translated
		_, err = cool_module.NewAnother_resource(ctx, "third", &cool_module.Another_resourceArgs{
			The_input: cool_module.Some_dataOutput(ctx, cool_module.Some_dataOutputArgs{
				The_input: first.The_output.ApplyT(func(the_output map[string][]cool_module.Output_item) (string, error) {
					return the_output["someKey"][0].Nested_output, nil
				}).(pulumi.StringOutput),
				Nested: cool_module.EntryArray{
					&cool_module.EntryArgs{
						Value: pulumi.String("fuzz"),
					},
				},
			}, nil).ApplyT(func(invoke cool_module.Some_dataResult) (string, error) {
				return invoke.Nested_output[0]["key"].Value, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
