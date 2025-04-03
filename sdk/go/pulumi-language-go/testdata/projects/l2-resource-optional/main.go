package main

import (
	"example.com/pulumi-simple-optional/sdk/go/v17/simpleoptional"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		resA, err := simple.NewResource(ctx, "resA", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		resB, err := simpleoptional.NewResource(ctx, "resB", &simpleoptional.ResourceArgs{
			Value: resA.Value,
		})
		if err != nil {
			return err
		}
		_, err = simpleoptional.NewResource(ctx, "resC", &simpleoptional.ResourceArgs{
			Value: resB.Value,
			Text:  nil,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
