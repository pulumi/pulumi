package main

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hookFun := func(args *pulumi.ResourceHookArgs) error {
			length := int(args.NewInputs["length"].NumberValue())
			ctx.Log.Info(fmt.Sprintf("fun was called with length = %d", length), nil)
			if args.Name != "res" {
				return fmt.Errorf("expected name to be 'res', got %q", args.Name)
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

		res, err := NewRandom(ctx,
			"res",
			&RandomArgs{
				Length: pulumi.Int(10),
			},
			pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
				BeforeCreate: []*pulumi.ResourceHook{hook},
			}))
		if err != nil {
			return err
		}

		hookFunComp := func(args *pulumi.ResourceHookArgs) error {
			ctx.Log.Info(fmt.Sprintf("funComp was called with args = %+v", args), nil)
			childId := args.NewOutputs["childId"].StringValue()
			if childId == "" {
				return errors.New("expected non empty childId")
			}
			ctx.Log.Info(fmt.Sprintf("funComp was called with child = %d\n", childId), nil)
			if args.Name != "comp" {
				return fmt.Errorf("expected name to be 'comp', got %q", args.Name)
			}
			if string(args.Type) != "testprovider:index:Component" {
				return fmt.Errorf("expected type to be 'testprovider:index:Component', got %q", args.Type)
			}
			return nil
		}
		hookComp, err := ctx.RegisterResourceHook("hook_fun_comp", hookFunComp, &pulumi.ResourceHookOptions{
			OnDryRun: false,
		})
		if err != nil {
			return err
		}
		_, err = NewComponent(ctx, "comp", &ComponentArgs{Length: pulumi.Int(10)},
			pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
				AfterCreate: []*pulumi.ResourceHook{hookComp},
			}),
		)
		if err != nil {
			return err
		}

		ctx.Export("name", res.ID())
		return nil
	})
}
