package main

import (
	usingdashes "example.com/pulumi-using-dashes/sdk/go/using-dashes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := usingdashes.NewDash(ctx, "main", &usingdashes.DashArgs{
			Stack: pulumi.String("dev"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
