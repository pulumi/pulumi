package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewResource(ctx, "noTimeouts", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "createOnly", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "5m"}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "updateOnly", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Timeouts(&pulumi.CustomTimeouts{Update: "10m"}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "deleteOnly", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Timeouts(&pulumi.CustomTimeouts{Delete: "3m"}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "allTimeouts", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "2m", Update: "4m", Delete: "1m"}))
		if err != nil {
			return err
		}
		return nil
	})
}
