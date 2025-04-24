package main

import (
	"git.example.org/pulumi-synthetic/resourceproperties"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		rt, err := resourceproperties.NewRoot(ctx, "rt", nil)
		if err != nil {
			return err
		}
		ctx.Export("trivial", rt)
		ctx.Export("simple", rt.Res1)
		ctx.Export("foo", rt.Res1.ApplyT(func(res1 *resourceproperties.Res1) (resourceproperties.Obj2, error) {
			return res1.Obj1.Res2.Obj2, nil
		}).(resourceproperties.Obj2Output))
		ctx.Export("complex", rt.Res1.ApplyT(func(res1 *resourceproperties.Res1) (*float64, error) {
			return &res1.Obj1.Res2.Obj2.Answer, nil
		}).(pulumi.Float64PtrOutput))
		return nil
	})
}
