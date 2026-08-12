package main

import (
	"fmt"
	"os/exec"

	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		hookTestFile := cfg.Require("hookTestFile")
		hookPreviewFile := cfg.Require("hookPreviewFile")
		createHook, err := ctx.RegisterResourceHook("createHook", func(args *pulumi.ResourceHookArgs) error {
			return exec.Command("touch", hookTestFile).Run()
		}, nil)
		if err != nil {
			return err
		}
		previewHook, err := ctx.RegisterResourceHook("previewHook", func(args *pulumi.ResourceHookArgs) error {
			return exec.Command("touch", fmt.Sprintf("%v_%v", hookPreviewFile, args.Name)).Run()
		}, &pulumi.ResourceHookOptions{OnDryRun: true})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "res", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ResourceHooks(&pulumi.ResourceHookBinding{BeforeCreate: []*pulumi.ResourceHook{createHook, previewHook}}))
		if err != nil {
			return err
		}
		return nil
	})
}
