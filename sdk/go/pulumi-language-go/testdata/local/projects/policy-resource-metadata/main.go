package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := simple.NewProvider(ctx, "prov", nil)
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "identity", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "protected", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "ignoreChanges", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.IgnoreChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "deleteBeforeReplace", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "secretOutput", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.AdditionalSecretOutputs([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "customTimeouts", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "5m"}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "explicitProvider", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		parent, err := simple.NewResource(ctx, "parent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "child", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent))
		if err != nil {
			return err
		}
		dependency, err := simple.NewResource(ctx, "dependency", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "dependsOn", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{
			dependency,
		}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "propertyDependency", &simple.ResourceArgs{
			Value: dependency.Value,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
