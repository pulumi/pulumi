package main

import (
	"example.com/pulumi-nestedcollections/sdk/go/v50/nestedcollections"
	"example.com/pulumi-nestedcollections/sdk/go/v50/nestedcollections/elementtype"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// A resource with deeply nested collection output properties: a list of lists of lists
		// of an object type and a map of maps of maps of strings.
		foo, err := nestedcollections.NewFoo(ctx, "foo", nil)
		if err != nil {
			return err
		}
		// A resource whose `elementType` property collides with the `ElementType()` method that
		// generated Go SDK types must implement.
		elem, err := elementtype.NewElementType(ctx, "elem", nil)
		if err != nil {
			return err
		}
		ctx.Export("secondProp", foo.ConditionSets.ApplyT(func(conditionSets [][][]nestedcollections.Bar) (string, error) {
			return conditionSets[0][0][1].Prop, nil
		}).(pulumi.StringOutput))
		ctx.Export("leaf", foo.PrivateEndpoint.ApplyT(func(privateEndpoint map[string]map[string]map[string]string) (string, error) {
			return privateEndpoint["outer"]["inner"]["leaf"], nil
		}).(pulumi.StringOutput))
		ctx.Export("elementType", elem.ElementType_.ElementTypeProp())
		return nil
	})
}
