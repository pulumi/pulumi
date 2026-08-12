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

// A promotion pipeline (dev -> staging -> prod) as a pulumi.Workflow.
//
// Drive it with config:
//
//	pulumi config set image app:v2          # a new image admits a new release cursor at dev
//	pulumi config set approveStaging true   # opens the dev -> staging gate
//	pulumi config set approveProd true      # opens the staging -> prod gate (after a 15s bake)
//	pulumi config set dropStaging true      # removes the staging node: its sub-state is destroyed
//
// Each `pulumi up` polls the gates once; parked releases wait, costing nothing, until a later up.
package main

import (
	"context"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		image := cfg.Get("image")
		if image == "" {
			image = "app:v1"
		}
		approveStaging := cfg.GetBool("approveStaging")
		approveProd := cfg.GetBool("approveProd")
		dropStaging := cfg.GetBool("dropStaging")

		// Each environment node is a mini Pulumi program: it reads the cursor's data as config,
		// manages its own resources, and its exports flow back into the cursor.
		deploy := func(env string) pulumi.RunFunc {
			return func(nctx *pulumi.Context) error {
				c := config.New(nctx, "workflow")
				img := c.Require("image")
				_, err := pulumi.NewStash(nctx, env, &pulumi.StashArgs{
					Input: pulumi.String(env + " runs " + img),
				})
				if err != nil {
					return err
				}
				nctx.Export("image", pulumi.String(img))
				nctx.Export("deployedAt", pulumi.String(time.Now().UTC().Format(time.RFC3339)))
				return nil
			}
		}

		_, err := pulumi.NewWorkflow(ctx, "release", func(g *pulumi.WorkflowGraph) error {
			dev := g.DefNode("dev", deploy("dev"))
			prod := g.DefNode("prod", deploy("prod"))

			if dropStaging {
				g.Edge(dev, prod, func(context.Context) (bool, error) {
					return approveProd, nil
				})
			} else {
				staging := g.DefNode("staging", deploy("staging"))
				g.Edge(dev, staging, func(context.Context) (bool, error) {
					return approveStaging, nil
				})
				g.Edge(staging, prod, func(cctx context.Context) (bool, error) {
					baked := time.Since(pulumi.WorkflowFrom(cctx).When) > 15*time.Second
					return approveProd && baked, nil
				})
			}

			g.Entry(dev, pulumi.Map{"image": pulumi.String(image)})
			return nil
		})
		return err
	})
}
