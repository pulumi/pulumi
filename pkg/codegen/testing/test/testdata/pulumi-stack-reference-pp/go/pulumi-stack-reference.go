package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pulumi.NewStackReference(ctx, "stackRef", &pulumi.StackReferenceArgs{
			Name: pulumi.String("foo/bar/dev"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
