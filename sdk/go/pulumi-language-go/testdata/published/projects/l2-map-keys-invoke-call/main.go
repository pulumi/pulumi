package main

import (
	"example.com/pulumi-component/sdk/go/v13/component"
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		callable, err := component.NewComponentCallable(ctx, "callable", &component.ComponentCallableArgs{
			Value: pulumi.String("unused"),
		})
		if err != nil {
			return err
		}
		ctx.Export("invokeResult", simpleinvoke.EchoMapOutput(ctx, simpleinvoke.EchoMapOutputArgs{
			StringMap: pulumi.StringMap{
				"my key":     pulumi.String("one"),
				"my.key":     pulumi.String("two"),
				"my-key":     pulumi.String("three"),
				"my_key":     pulumi.String("four"),
				"MY_KEY":     pulumi.String("five"),
				"myKey":      pulumi.String("six"),
				"__type":     pulumi.String("seven"),
				"__internal": pulumi.String("eight"),
			},
		}, nil).StringMap())
		callEchoMap, err := callable.EchoMap(ctx, &component.ComponentCallableEchoMapArgs{
			StringMap: pulumi.StringMap{
				"my key":     pulumi.String("one"),
				"my.key":     pulumi.String("two"),
				"my-key":     pulumi.String("three"),
				"my_key":     pulumi.String("four"),
				"MY_KEY":     pulumi.String("five"),
				"myKey":      pulumi.String("six"),
				"__type":     pulumi.String("seven"),
				"__internal": pulumi.String("eight"),
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("callResult", callEchoMap.ApplyT(func(call component.ComponentCallableEchoMapResult) (map[string]string, error) {
			return call.StringMap, nil
		}).(pulumi.StringMapOutput))
		return nil
	})
}
