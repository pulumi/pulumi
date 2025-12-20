package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		parent, err := simple.NewResource(ctx, "parent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		aliasURN, err := simple.NewResource(ctx, "aliasURN", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Aliases([]pulumi.Alias{pulumi.Alias{URN: pulumi.URN("urn:pulumi:test::l2-resource-option-alias::simple:index:Resource::aliasURN")}}), pulumi.Parent(parent))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "aliasNewName", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Aliases([]pulumi.Alias{pulumi.Alias{Name: pulumi.String("aliasName")}}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "aliasNoParent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Aliases([]pulumi.Alias{pulumi.Alias{NoParent: pulumi.Bool(true)}}), pulumi.Parent(parent))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "aliasParent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Aliases([]pulumi.Alias{pulumi.Alias{Parent: aliasURN}}), pulumi.Parent(parent))
		if err != nil {
			return err
		}
		return nil
	})
}
