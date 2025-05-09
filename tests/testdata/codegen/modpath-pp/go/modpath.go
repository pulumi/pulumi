package main

import (
	other "git.example.org/thirdparty/sdk/go/pkg"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := other.NewThing(ctx, "thing", &other.ThingArgs{
			Idea: "myIdea",
		})
		if err != nil {
			return err
		}
		return nil
	})
}
