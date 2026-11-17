package main

import (
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rivo/uniseg"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		aString := cfg.Require("aString")
		ctx.Export("lengthResult", pulumi.Int(uniseg.GraphemeClusterCount(aString)))
		ctx.Export("splitResult", pulumi.ToStringArray(strings.Split(aString, "-")))
		ctx.Export("joinResult", pulumi.String(strings.Join(strings.Split(aString, "-"), "|")))
		ctx.Export("interpolateResult", pulumi.Sprintf("prefix-%v", aString))
		return nil
	})
}
