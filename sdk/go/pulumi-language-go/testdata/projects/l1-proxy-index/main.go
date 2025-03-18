package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		object := cfg.Require("object")
		l := pulumi.ToSecret([]float64{
			1,
		}).(pulumi.Float64ArrayOutput)
		m := pulumi.ToSecret(map[string]interface{}{
			"key": true,
		}).(pulumi.BoolMapOutput)
		c := pulumi.ToSecret(object).(pulumi.StringMapOutput)
		o := pulumi.ToSecret(map[string]interface{}{
			"property": "value",
		}).(pulumi.StringMapOutput)
		ctx.Export("l", l.ApplyT(func(l []float64) (float64, error) {
			return l[0], nil
		}).(pulumi.Float64Output))
		ctx.Export("m", m.ApplyT(func(m map[string]interface{}) (bool, error) {
			return m.Key, nil
		}).(pulumi.BoolOutput))
		ctx.Export("c", c.ApplyT(func(c map[string]interface{}) (*string, error) {
			return &c.Property, nil
		}).(pulumi.StringPtrOutput))
		ctx.Export("o", o.ApplyT(func(o map[string]interface{}) (string, error) {
			return o.Property, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
