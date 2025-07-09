package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hookFun := func(args *pulumi.ResourceHookArgs) error {
			length := int(args.NewInputs["length"].NumberValue())
			ctx.Log.Info(fmt.Sprintf("fun was called with length = %d\n", length), nil)
			if args.Name != "username" {
				return fmt.Errorf("expected name to be 'username', got %q", args.Name)
			}
			if string(args.Type) != "testprovider:index:Random" {
				return fmt.Errorf("expected type to be 'testprovider:index:Random', got %q", args.Type)
			}
			return nil
		}
		hook, err := ctx.RegisterResourceHook("myhook", hookFun, &pulumi.ResourceHookOptions{
			OnDryRun: false,
		})
		if err != nil {
			return err
		}

		username, err := NewRandom(ctx,
			"username",
			&RandomArgs{
				Length: pulumi.Int(10),
			},
			pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
				BeforeCreate: []*pulumi.ResourceHook{hook},
			}))
		if err != nil {
			return err
		}

		ctx.Export("name", username.ID())
		return nil
	})
}
