// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hookFun := func(args *pulumi.ResourceHookArgs) error {
			echoProp := args.NewInputs["echo"]
			if !echoProp.IsSecret() {
				return fmt.Errorf("expected echo to be secret")
			}
			val := echoProp.SecretValue().Element.StringValue()
			if val != "hello secret" {
				return fmt.Errorf("expected 'hello secret', got %q", val)
			}
			ctx.Log.Info("hook called", nil)
			return nil
		}

		hook, err := ctx.RegisterResourceHook("secret_hook", hookFun, nil)
		if err != nil {
			return err
		}

		_, err = NewEcho(ctx, "echo", &EchoArgs{
			Echo: pulumi.ToSecret(pulumi.String("hello secret")).(pulumi.StringOutput),
		}, pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
			BeforeCreate: []*pulumi.ResourceHook{hook},
		}))

		return err
	})
}
