// Copyright 2026, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-provider/sdks/go/random/v3/random"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const projectName = "stale-parameterized-packageref"

func main() {
	ctx := context.Background()

	if err := preview(ctx, "stack-1", func(ctx *pulumi.Context) error {
		_, err := random.NewPassword(ctx, "my-password", &random.PasswordArgs{
			Length: pulumi.Float64(16),
		})
		return err
	}); err != nil {
		fail("first preview failed", err)
	}
	fmt.Println("First preview succeeded")

	if err := preview(ctx, "stack-2", func(ctx *pulumi.Context) error {
		_, err := random.NewUuid(ctx, "my-uuid", &random.UuidArgs{})
		return err
	}); err != nil {
		fail("second preview failed", err)
	}
	fmt.Println("Second preview succeeded")
}

func preview(ctx context.Context, stackName string, program pulumi.RunFunc) error {
	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, program)
	if err != nil {
		return fmt.Errorf("create stack: %w", err)
	}
	defer func() {
		if err := stack.Workspace().RemoveStack(ctx, stack.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "remove stack %s: %v\n", stack.Name(), err)
		}
	}()
	if _, err := stack.Preview(ctx); err != nil {
		return fmt.Errorf("preview: %w", err)
	}
	return nil
}

func fail(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}
