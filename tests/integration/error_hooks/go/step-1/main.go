package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type FlakyCreate struct {
	pulumi.CustomResourceState
}

func NewFlakyCreate(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*FlakyCreate, error) {
	var resource FlakyCreate
	err := ctx.RegisterResource("testprovider:index:FlakyCreate", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		onError := func(args *pulumi.ErrorHookArgs) (bool, error) {
			ctx.Log.Info(fmt.Sprintf("onError was called for %s (%s)", args.Name, args.FailedOperation), nil)
			if args.Name != "res" {
				return false, fmt.Errorf("expected name to be 'res', got %q", args.Name)
			}
			if string(args.Type) != "testprovider:index:FlakyCreate" {
				return false, fmt.Errorf("expected type to be 'testprovider:index:FlakyCreate', got %q", args.Type)
			}
			if args.FailedOperation != "create" {
				return false, fmt.Errorf("expected failed operation 'create', got %q", args.FailedOperation)
			}
			if len(args.Errors) == 0 {
				return false, fmt.Errorf("expected at least one error message")
			}
			return true, nil
		}
		hook, err := ctx.RegisterErrorHook("onError", onError)
		if err != nil {
			return err
		}

		_, err = NewFlakyCreate(ctx, "res", pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
			OnError: []*pulumi.ErrorHook{hook},
		}))
		return err
	})
}
