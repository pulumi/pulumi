package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	ctx := context.Background()

	projectName := "random-password"
	stackName := "dev"

	// Define the Pulumi program inline
	pulumiProgram := func(ctx *pulumi.Context) error {
		pw, err := random.NewRandomPassword(ctx, "my-password", &random.RandomPasswordArgs{
			Length:  pulumi.Int(16),
			Special: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("password", pw.Result)
		return nil
	}

	// Create or select a stack
	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, pulumiProgram)
	if err != nil {
		panic(fmt.Errorf("failed to create or select stack: %w", err))
	}

	fmt.Println("Running pulumi up...")
	res, err := s.Up(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to update stack: %w", err))
	}

	fmt.Printf("Password: %s\n", res.Outputs["password"].Value)
}
