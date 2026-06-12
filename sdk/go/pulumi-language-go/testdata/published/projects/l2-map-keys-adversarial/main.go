package main

import (
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := primitive.NewResource(ctx, "res", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(false),
			Float:   pulumi.Float64(2.17),
			Integer: pulumi.Int(-12),
			String:  pulumi.String("adversarial"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(0),
				pulumi.Float64(1),
			},
			BooleanMap: pulumi.BoolMap{
				"__type":     pulumi.Bool(true),
				"__internal": pulumi.Bool(false),
				"__provider": pulumi.Bool(true),
				"__version":  pulumi.Bool(false),
				"":           pulumi.Bool(true),
				"Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \x1b (escape), \a (bell), \x00 (null), \U000e0021 (tag space)": pulumi.Bool(false),
				"Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash":      pulumi.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		invokeResult := primitive.InvokeOutput(ctx, primitive.InvokeOutputArgs{
			Boolean: pulumi.Bool(false),
			Float:   pulumi.Float64(2.17),
			Integer: pulumi.Int(-12),
			String:  pulumi.String("adversarial"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(0),
				pulumi.Float64(1),
			},
			BooleanMap: pulumi.BoolMap{
				"__type":     pulumi.Bool(true),
				"__internal": pulumi.Bool(false),
				"__provider": pulumi.Bool(true),
				"__version":  pulumi.Bool(false),
				"":           pulumi.Bool(true),
				"Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \x1b (escape), \a (bell), \x00 (null), \U000e0021 (tag space)": pulumi.Bool(false),
				"Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash":      pulumi.Bool(true),
			},
		}, nil)
		ctx.Export("resourceBooleanMap", res.BooleanMap)
		ctx.Export("invokeBooleanMap", invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (map[string]bool, error) {
			return invokeResult.BooleanMap, nil
		}).(pulumi.BoolMapOutput))
		return nil
	})
}
