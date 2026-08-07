package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		names := []string{
			"alpha",
			"beta",
			"gamma",
		}
		var forResult0 []string
		for _, n := range names {
			forResult0 = append(forResult0, fmt.Sprintf("prefix-%v", n))
		}
		ctx.Export("prefixed", pulumi.ToStringArray(forResult0))
		var forResult1 []string
		for _, n := range names {
			if n != "beta" {
				forResult1 = append(forResult1, n)
			}
		}
		ctx.Export("filtered", pulumi.ToStringArray(forResult1))
		var forResult2 []string
		for i, n := range names {
			forResult2 = append(forResult2, fmt.Sprintf("%v:%v", i, n))
		}
		ctx.Export("indexed", pulumi.ToStringArray(forResult2))
		return nil
	})
}
