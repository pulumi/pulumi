package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("stackOutput", pulumi.String(ctx.Stack()))
		ctx.Export("projectOutput", pulumi.String(ctx.Project()))
		ctx.Export("organizationOutput", pulumi.String(ctx.Organization()))
		return nil
	})
}
