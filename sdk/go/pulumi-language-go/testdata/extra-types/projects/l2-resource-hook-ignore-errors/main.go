package main

import (
	"os/exec"

	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		failingHook, err := ctx.RegisterResourceHook("failingHook", func(args *pulumi.ResourceHookArgs) error {
			return exec.Command("false").Run()
		}, &pulumi.ResourceHookOptions{IgnoreErrors: true})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "res", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ResourceHooks(&pulumi.ResourceHookBinding{AfterCreate: []*pulumi.ResourceHook{failingHook}}))
		if err != nil {
			return err
		}
		return nil
	})
}
