package main

import (
	"example.com/pulumi-conformance-component/sdk/go/v22/conformancecomponent"
	"example.com/pulumi-replaceonchanges/sdk/go/v25/replaceonchanges"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Stage 0: Initial resource creation
		// Scenario 1: Schema-based replaceOnChanges on replaceProp
		_, err := replaceonchanges.NewResourceA(ctx, "schemaReplace", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		// Scenario 2: Option-based replaceOnChanges on value
		_, err = replaceonchanges.NewResourceB(ctx, "optionReplace", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 3: Both schema and option - will change value
		_, err = replaceonchanges.NewResourceA(ctx, "bothReplaceValue", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 4: Both schema and option - will change replaceProp
		_, err = replaceonchanges.NewResourceA(ctx, "bothReplaceProp", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 5: No replaceOnChanges - baseline update
		_, err = replaceonchanges.NewResourceB(ctx, "regularUpdate", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		// Scenario 6: replaceOnChanges set but no change
		_, err = replaceonchanges.NewResourceB(ctx, "noChange", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 7: replaceOnChanges on value, but only replaceProp changes
		_, err = replaceonchanges.NewResourceA(ctx, "wrongPropChange", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 8: Multiple properties in replaceOnChanges array
		_, err = replaceonchanges.NewResourceA(ctx, "multiplePropReplace", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
			"replaceProp",
		}))
		if err != nil {
			return err
		}
		// Remote component with replaceOnChanges
		_, err = conformancecomponent.NewSimple(ctx, "remoteWithReplace", &conformancecomponent.SimpleArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Keep a simple resource so all expected plugins are required.
		_, err = simple.NewResource(ctx, "simpleResource", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
