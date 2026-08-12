package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		var anObject map[string]interface{}
		cfg.RequireObject("anObject", &anObject)
		var anyObject interface{}
		cfg.RequireObject("anyObject", &anyObject)
		l := pulumi.ToSecret([]int{
			1,
		}).(pulumi.IntArrayOutput)
		m := pulumi.ToSecret(map[string]bool{
			"key": true,
		}).(pulumi.BoolMapOutput)
		c := pulumi.ToSecret(anObject).(pulumi.MapOutput)
		o := pulumi.ToSecret(map[string]string{
			"property": "value",
		}).(pulumi.StringMapOutput)
		a := pulumi.ToSecret(pulumi.Any(anyObject)).(pulumi.AnyOutput)
		ctx.Export("l", l.ApplyT(func(l []int) (int, error) {
			return l[0], nil
		}).(pulumi.IntOutput))
		ctx.Export("m", m.ApplyT(func(m map[string]bool) (bool, error) {
			return m["key"], nil
		}).(pulumi.BoolOutput))
		ctx.Export("c", c.ApplyT(func(c map[string]interface{}) (*string, error) {
			val := c["property"].(string)
			return &val, nil
		}).(pulumi.StringPtrOutput))
		ctx.Export("o", o.ApplyT(func(o map[string]string) (string, error) {
			return o["property"], nil
		}).(pulumi.StringOutput))
		ctx.Export("a", a.ApplyT(func(a interface{}) (interface{}, error) {
			return a.(map[string]interface{})["property"], nil
		}).(pulumi.AnyOutput))
		return nil
	})
}
