package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myComponent := &pulumi.ResourceState{}
		err := ctx.RegisterComponentResourceV2("my:custom:Component", "myComponent", pulumi.Map{
			"aNumber": pulumi.Any(42),
			"aString": pulumi.Any("hello"),
		}, myComponent)
		if err != nil {
			return err
		}
		return nil
	})
}
