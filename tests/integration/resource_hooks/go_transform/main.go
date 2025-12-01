package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hookFun := func(args *pulumi.ResourceHookArgs) error {
			if args.Name == "res" {
				length := int(args.NewInputs["length"].NumberValue())
				ctx.Log.Info(fmt.Sprintf("fun was called with length = %d", length), nil)
				if args.Name != "res" {
					return fmt.Errorf("expected name to be 'res', got %q", args.Name)
				}
				if string(args.Type) != "testprovider:index:Random" {
					return fmt.Errorf("expected type to be 'testprovider:index:Random', got %q", args.Type)
				}
			} else if args.Name == "comp" {
				childId := args.NewOutputs["childId"].StringValue()
				ctx.Log.Info(fmt.Sprintf("fun_comp was called with child = %s", childId), nil)
				if childId == "" {
					return errors.New("expected non empty childId")
				}
				if args.Name != "comp" {
					return fmt.Errorf("expected name to be 'comp', got %q", args.Name)
				}
				if string(args.Type) != "testprovider:index:Component" {
					return fmt.Errorf("expected type to be 'testprovider:index:Component', got %q", args.Type)
				}
			}
			return fmt.Errorf("got unexpected component name: %s", args.Name)
		}

		hook, err := ctx.RegisterResourceHook("hook_fun", hookFun, &pulumi.ResourceHookOptions{
			OnDryRun: false,
		})
		if err != nil {
			return err
		}

		transform := func(ctx context.Context, args *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
			opts := args.Opts

			var existingAfterCreate []*pulumi.ResourceHook
			if opts.Hooks != nil && opts.Hooks.AfterCreate != nil {
				existingAfterCreate = opts.Hooks.AfterCreate
			}

			newHooks := &pulumi.ResourceHookBinding{
				AfterCreate: append(existingAfterCreate, hook),
			}

			if opts.Hooks != nil {
				newHooks.BeforeCreate = opts.Hooks.BeforeCreate
				newHooks.BeforeUpdate = opts.Hooks.BeforeUpdate
				newHooks.AfterUpdate = opts.Hooks.AfterUpdate
				newHooks.BeforeDelete = opts.Hooks.BeforeDelete
				newHooks.AfterDelete = opts.Hooks.AfterDelete
			}

			opts.Hooks = newHooks

			return &pulumi.ResourceTransformResult{
				Props: args.Props,
				Opts:  opts,
			}
		}

		res, err := NewRandom(ctx,
			"res",
			&RandomArgs{
				Length: pulumi.Int(10),
			},
			pulumi.Transforms([]pulumi.ResourceTransform{transform}))
		if err != nil {
			return err
		}

		_, err = NewComponent(ctx, "comp", &ComponentArgs{Length: pulumi.Int(7)},
			pulumi.Transforms([]pulumi.ResourceTransform{transform}),
		)
		if err != nil {
			return err
		}

		ctx.Export("name", res.ID())
		return nil
	})
}
