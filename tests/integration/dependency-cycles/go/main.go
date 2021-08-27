// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi-random/sdk/v3/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// 	a1 := res("a1", "")
// 	a2 := res("a2", "a1", "a1")
// 	a3 := res("a3", "a1")
// 	b1 := res("b1", "a2")
// 	dg := NewDependencyGraph([]*resource.State{a1, a2, a3, b1})
// 	// b1 should depend on a2 its parent
// 	assert.Contains(t, dg.DependenciesOf(b1), a2)
// 	// but a2 should not depend on b1
//      assert.NotContains(t, dg.DependenciesOf(a2), b1)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		args := &random.RandomPetArgs{Length: pulumi.Int(1)}
		a1, err := random.NewRandomPet(ctx, "a1", args)
		if err != nil {
			return err
		}
		a2, err := random.NewRandomPet(ctx, "a2", args, pulumi.Parent(a1), pulumi.DependsOn([]pulumi.Resource{a1}))
		if err != nil {
			return err
		}
		a3, err := random.NewRandomPet(ctx, "a3", &random.RandomPetArgs{Length: pulumi.Int(1)}, pulumi.Parent(a1))
		if err != nil {
			return err
		}
		b1, err := random.NewRandomPet(ctx, "b1", &random.RandomPetArgs{Length: pulumi.Int(1)}, pulumi.Parent(a2))
		if err != nil {
			return err
		}

		ctx.Export("a1_URN", a1.URN())
		ctx.Export("a2_URN", a2.URN())
		ctx.Export("a3_URN", a3.URN())
		ctx.Export("b1_URN", b1.URN())

		return nil
	})
}
