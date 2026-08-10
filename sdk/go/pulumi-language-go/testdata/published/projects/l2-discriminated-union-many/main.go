package main

import (
	"example.com/pulumi-discriminated-union-many/sdk/go/v49/discriminatedunionmany"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := discriminatedunionmany.NewExample(ctx, "example1", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant1Args{
				DiscriminantKind: pulumi.String("variant1"),
				Payload:          pulumi.String("p1"),
				Extra:            pulumi.String("e1"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example2", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant2Args{
				DiscriminantKind: pulumi.String("variant2"),
				Payload:          pulumi.String("p2"),
				Extra:            pulumi.String("e2"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example3", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant3Args{
				DiscriminantKind: pulumi.String("variant3"),
				Payload:          pulumi.String("p3"),
				Count:            pulumi.Int(3),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example4", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant4Args{
				DiscriminantKind: pulumi.String("variant4"),
				Payload:          pulumi.String("p4"),
				Enabled:          pulumi.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example5", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant5Args{
				DiscriminantKind: pulumi.String("variant5"),
				Payload:          pulumi.String("p5"),
				Label:            pulumi.String("l5"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example6", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant6Args{
				DiscriminantKind: pulumi.String("variant6"),
				Payload:          pulumi.String("p6"),
				Code:             pulumi.Int(6),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example7", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant7Args{
				DiscriminantKind: pulumi.String("variant7"),
				Payload:          pulumi.String("p7"),
				Message:          pulumi.String("m7"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example8", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant8Args{
				DiscriminantKind: pulumi.String("variant8"),
				Payload:          pulumi.String("p8"),
				Size:             pulumi.Int(8),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example9", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant9Args{
				DiscriminantKind: pulumi.String("variant9"),
				Payload:          pulumi.String("p9"),
				Flag:             pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example10", &discriminatedunionmany.ExampleArgs{
			UnionOf: &discriminatedunionmany.Variant10Args{
				DiscriminantKind: pulumi.String("variant10"),
				Payload:          pulumi.String("p10"),
				Note:             pulumi.String("n10"),
			},
		})
		if err != nil {
			return err
		}
		// A SubsetExample's unionOf is typed as a 3-variant subset union. We should be
		// able to assign that output to an Example's unionOf, which is typed as the
		// full 10-variant union.
		subset1, err := discriminatedunionmany.NewSubsetExample(ctx, "subset1", &discriminatedunionmany.SubsetExampleArgs{
			UnionOf: &discriminatedunionmany.Variant3Args{
				DiscriminantKind: pulumi.String("variant3"),
				Payload:          pulumi.String("sp"),
				Count:            pulumi.Int(33),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunionmany.NewExample(ctx, "example11", &discriminatedunionmany.ExampleArgs{
			UnionOf: subset1.UnionOf,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
