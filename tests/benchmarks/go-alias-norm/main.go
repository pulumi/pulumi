// Stress-test the engine handling many resources with many aliases.

package main

import (
	"fmt"

	random "github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		mode := conf.Require("mode")
		n := conf.RequireInt("n")

		parent, err := makeResource(ctx, nil, nil, 0, "parent")
		if err != nil {
			return err
		}

		var prev []*random.RandomInteger
		for i := 0; i < n; i++ {
			r, err := makeResource(ctx, parent, prev, i, mode)
			if err != nil {
				return err
			}
			prev = append(prev, r)
		}
		return nil
	})
}

func nameResource(i int, mode string) string {
	return fmt.Sprintf("resource-%s-%d", mode, i)
}

func makeResource(
	ctx *pulumi.Context,
	parent *random.RandomInteger,
	prev []*random.RandomInteger,
	i int,
	mode string,
) (*random.RandomInteger, error) {
	name := nameResource(i, mode)
	opts := []pulumi.ResourceOption{}
	if len(prev) != 0 {
		deps := []pulumi.Resource{}
		for _, p := range prev {
			deps = append(deps, p)
		}
		opts = append(opts, pulumi.DependsOn(deps))
	} else if parent != nil {
		opts = append(opts, pulumi.DependsOn([]pulumi.Resource{parent}))
	}
	if mode == "alias" {
		alias := pulumi.Alias{
			Name:     pulumi.String(nameResource(i, "new")),
			NoParent: pulumi.Bool(true),
		}
		opts = append(opts, pulumi.Aliases([]pulumi.Alias{alias}))
		opts = append(opts, pulumi.Parent(parent))
	}

	if len(prev) != 0 {
		ints := []interface{}{}
		for _, p := range prev {
			ints = append(ints, p.Result)
		}

		var derived pulumi.IntOutput = pulumi.All(ints...).ApplyT(func(data []interface{}) int {
			s := 10
			return s
		}).(pulumi.IntOutput)

		return random.NewRandomInteger(ctx,
			name,
			&random.RandomIntegerArgs{
				Min: derived,
				Max: derived,
			},
			opts...)

	} else {
		return random.NewRandomInteger(ctx,
			name,
			&random.RandomIntegerArgs{
				Min: pulumi.Int(0),
				Max: pulumi.Int(100),
			},
			opts...)
	}
}
