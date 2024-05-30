package main

import (
	eventgrid "github.com/pulumi/pulumi-azure-native-sdk/eventgrid/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := eventgrid.NewEventSubscription(ctx, "example", &eventgrid.EventSubscriptionArgs{
			ExpirationTimeUtc: pulumi.String("example"),
			Scope:             pulumi.String("example"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
