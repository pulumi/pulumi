package main

import (
	"example.com/pulumi-keywords/sdk/go/v20/keywords"
	"example.com/pulumi-keywords/sdk/go/v20/keywords/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		firstResource, err := keywords.NewSomeResource(ctx, "firstResource", &keywords.SomeResourceArgs{
			Builtins: pulumi.String("builtins"),
			Lambda:   pulumi.String("lambda"),
			Property: pulumi.String("property"),
		})
		if err != nil {
			return err
		}
		_, err = keywords.NewSomeResource(ctx, "secondResource", &keywords.SomeResourceArgs{
			Builtins: firstResource.Builtins,
			Lambda:   firstResource.Lambda,
			Property: firstResource.Property,
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewSomeResource(ctx, "lambdaModuleResource", &lambda.SomeResourceArgs{
			Builtins: pulumi.String("builtins"),
			Lambda:   pulumi.String("lambda"),
			Property: pulumi.String("property"),
		})
		if err != nil {
			return err
		}
		_, err = keywords.NewLambda(ctx, "lambdaResource", &keywords.LambdaArgs{
			Builtins: pulumi.String("builtins"),
			Lambda:   pulumi.String("lambda"),
			Property: pulumi.String("property"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
