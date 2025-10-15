package main

import (
	"example.com/pulumi-secret/sdk/go/v14/secret"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := secret.NewResource(ctx, "res", &secret.ResourceArgs{
			Private: pulumi.String("closed"),
			Public:  pulumi.String("open"),
			PrivateData: &secret.DataArgs{
				Private: pulumi.String("closed"),
				Public:  pulumi.String("open"),
			},
			PublicData: &secret.DataArgs{
				Private: pulumi.String("closed"),
				Public:  pulumi.String("open"),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
