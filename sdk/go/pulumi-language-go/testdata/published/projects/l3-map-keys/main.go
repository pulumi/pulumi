package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cmp, err := NewMyComponent(ctx, "cmp", &MyComponentArgs{
			BooleanMap: map[string]bool{
				"my key":     false,
				"my.key":     true,
				"my-key":     false,
				"my_key":     true,
				"MY_KEY":     false,
				"myKey":      true,
				"__type":     true,
				"__internal": false,
				"__provider": true,
				"__version":  false,
				"":           true,
				"Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \x1b (escape), \a (bell), \x00 (null), \U000e0021 (tag space)": false,
				"Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash":      true,
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("resourceBooleanMap", cmp.BooleanMap)
		return nil
	})
}
