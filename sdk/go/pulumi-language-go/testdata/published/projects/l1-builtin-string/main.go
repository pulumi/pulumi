package main

import (
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		aString := cfg.Require("aString")
		ctx.Export("lengthOutput", pulumi.Int(len(aString)))
		ctx.Export("splitOutput", pulumi.StringArray(strings.Split(aString, "-")))
		ctx.Export("joinOutput", pulumi.String(strings.Join(strings.Split(aString, "-"), "|")))
		ctx.Export("interpolateOutput", pulumi.Sprintf("prefix-%v", aString))
		return nil
	})
}
