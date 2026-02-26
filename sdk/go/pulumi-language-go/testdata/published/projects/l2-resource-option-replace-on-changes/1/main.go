package main

import (
	"example.com/pulumi-conformance-component/sdk/go/v22/conformancecomponent"
	"example.com/pulumi-replaceonchanges/sdk/go/v25/replaceonchanges"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Stage 1: Change properties to trigger replacements
		// Scenario 1: Change replaceProp → REPLACE (schema triggers)
		_, err := replaceonchanges.NewResourceA(ctx, "schemaReplace", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		// Scenario 2: Change value → REPLACE (option triggers)
		_, err = replaceonchanges.NewResourceB(ctx, "optionReplace", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(false),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 3: Change value → REPLACE (option on value triggers)
		_, err = replaceonchanges.NewResourceA(ctx, "bothReplaceValue", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(false),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 4: Change replaceProp → REPLACE (schema on replaceProp triggers)
		_, err = replaceonchanges.NewResourceA(ctx, "bothReplaceProp", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(false),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 5: Change value → UPDATE (no replaceOnChanges)
		_, err = replaceonchanges.NewResourceB(ctx, "regularUpdate", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		// Scenario 6: No change → SAME (no operation)
		_, err = replaceonchanges.NewResourceB(ctx, "noChange", &replaceonchanges.ResourceBArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 7: Change replaceProp (not value) → UPDATE (marked property unchanged)
		_, err = replaceonchanges.NewResourceA(ctx, "wrongPropChange", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(true),
			ReplaceProp: pulumi.Bool(false),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		// Scenario 8: Change value → REPLACE (multiple properties marked)
		_, err = replaceonchanges.NewResourceA(ctx, "multiplePropReplace", &replaceonchanges.ResourceAArgs{
			Value:       pulumi.Bool(false),
			ReplaceProp: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
			"replaceProp",
		}))
		if err != nil {
			return err
		}
		// Remote component: change value → REPLACE
		_, err = conformancecomponent.NewSimple(ctx, "remoteWithReplace", &conformancecomponent.SimpleArgs{
			Value: pulumi.Bool(false),
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
