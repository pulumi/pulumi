package main

import (
	"os/exec"

	"example.com/pulumi-flaky/sdk/go/v46/flaky"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		hookTestFile := cfg.Require("hookTestFile")
		retryHook, err := ctx.RegisterErrorHook("retryHook", func(args *pulumi.ErrorHookArgs) (bool, error) {
			return exec.Command("touch", hookTestFile).Run() == nil, nil
		})
		if err != nil {
			return err
		}
		_, err = flaky.NewFlakyCreate(ctx, "res", nil, pulumi.ResourceHooks(&pulumi.ResourceHookBinding{OnError: []*pulumi.ErrorHook{retryHook}}))
		if err != nil {
			return err
		}
		return nil
	})
}
