// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// A small but non-trivial workflow: one release cursor per region deploys its region, and once every
// region is healthy and shipping is approved, a join merges them into a single production rollout.
//
//	east ─┐
//	      ├─ ship (join, gated on approve) ─▶ prod
//	west ─┘
package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		sha := cfg.Require("sha")
		approve := cfg.GetBool("approve")

		deploy := func(name string) workflow.NodeFunc {
			return func(ctx *pulumi.Context, c *workflow.Cursor) error {
				id, err := random.NewRandomId(ctx, name, &random.RandomIdArgs{
					ByteLength: pulumi.Int(4),
					Keepers:    pulumi.StringMap{"sha": pulumi.String(workflow.Require[string](c, "sha"))},
				})
				if err != nil {
					return err
				}
				c.Set("deployment", id.Hex)
				c.Set("healthy", true)
				return nil
			}
		}
		wf, err := workflow.New(ctx, "rollout", func(w *workflow.Context) error {
			east := w.Node("east", deploy("east"))
			west := w.Node("west", deploy("west"))
			prod := w.Node("prod", deploy("prod"))
			w.Cursor(east, "east", pulumi.Map{"sha": pulumi.String(sha), "region": pulumi.String("east")})
			w.Cursor(west, "west", pulumi.Map{"sha": pulumi.String(sha), "region": pulumi.String("west")})

			// A branch releases its region once it is healthy and shipping is approved; the branch that
			// completes the join stamps its approval on its cursor.
			gate := func(_ context.Context, c *workflow.Cursor) (bool, error) {
				c.Set("approved", true)
				c.Delete("region") // The merged rollout is region-less.
				return approve && workflow.Require[bool](c, "healthy"), nil
			}
			w.Join("ship", workflow.JoinMap{east: gate, west: gate}, prod,
				func(_ context.Context, in []workflow.Candidate) (*workflow.Merged, error) {
					values := map[string]any{}
					for _, cand := range in {
						if !workflow.Require[bool](cand.Cursor, "approved") {
							return nil, fmt.Errorf("candidate %q is not approved", cand.Cursor.Name())
						}
						values["sha"] = workflow.Require[string](cand.Cursor, "sha")
						from := workflow.Require[string](cand.Cursor, "deployment")
						values["from-"+cand.From.Name()] = from
					}
					return &workflow.Merged{Name: "release", Values: values}, nil
				})
			return nil
		})
		if err != nil {
			return err
		}
		ctx.Export("cursors", wf.Cursors)
		ctx.Export("diagram", wf.Diagram)
		return nil
	})
}
