package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("empty", pulumi.Map{})
		ctx.Export("strings", pulumi.StringMap{
			"greeting": pulumi.String("Hello, world!"),
			"farewell": pulumi.String("Goodbye, world!"),
		})
		ctx.Export("adversarialStrings", pulumi.StringMap{
			"__type":       pulumi.String("dunder type"),
			"__internal":   pulumi.String("dunder internal"),
			"__provider":   pulumi.String("dunder provider"),
			"__version":    pulumi.String("dunder version"),
			"":             pulumi.String("empty key"),
			"empty value":  pulumi.String(""),
			"dunder value": pulumi.String("__dunder"),
			"Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \x1b (escape), \a (bell), \x00 (null), \U000e0021 (tag space)": pulumi.String("Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \x1b (escape), \a (bell), \x00 (null), \U000e0021 (tag space)"),
		})
		ctx.Export("numbers", pulumi.IntMap{
			"1": pulumi.Int(1),
			"2": pulumi.Int(2),
		})
		ctx.Export("keys", pulumi.IntMap{
			"my.key": pulumi.Int(1),
			"my-key": pulumi.Int(2),
			"my_key": pulumi.Int(3),
			"MY_KEY": pulumi.Int(4),
			"mykey":  pulumi.Int(5),
			"MYKEY":  pulumi.Int(6),
		})
		return nil
	})
}
