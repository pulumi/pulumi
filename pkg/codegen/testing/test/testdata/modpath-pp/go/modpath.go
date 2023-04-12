package main

import (
	"git.example.org/thirdparty/sdk/go/pkg"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pkg.NewThing(ctx, "thing", &pkg.ThingArgs{
			Idea: pulumi.String("myIdea"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
