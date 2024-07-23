package main

import (
	"github.com/pulumi/pulumi-std/sdk/go/std"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := std.Replace(ctx, &std.ReplaceArgs{
			Text: std.Upper(ctx, &std.UpperArgs{
				Input: "hello_world",
			}, nil).Result,
			Search:  "_",
			Replace: "-",
		}, nil)
		if err != nil {
			return err
		}
		return nil
	})
}
