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

// A Pulumi program that registers a RESOURCE TRANSFORM — the one place a program serves
// an inbound RPC rather than only dialing out.
//
// RegisterResourceTransform (unlike the deprecated, in-process RegisterStackTransformation)
// runs the transform in the *program* via a callback: the SDK stands up a callback gRPC
// server, hands the engine its address, and the engine dials back into the program for every
// resource registration. That callback server binds 127.0.0.1 on a kernel-chosen port and
// advertises that literal address (sdk/go/pulumi/callback.go), which is only reachable if
// the engine shares the program's loopback.
//
// So this program's success is a direct probe of program<->engine topology:
//   - engine and program in one netns (CRI sandbox; local non-pod runs) -> transform applies
//   - program in its own netns (docker/nerdctl pod mode, where it is a sibling on a bridge)
//     -> the engine cannot reach 127.0.0.1:<port> and the transform cannot run
//
// The transform sets `prefix`, so the resulting pet name is self-evidently transformed or
// not: a name starting with "transformed-" proves the engine reached back into the program.
package main

import (
	"context"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// transformedPrefix is injected by the transform and shows up in the pet's name, so the
// stack output alone distinguishes "the transform ran" from "the transform was skipped".
const transformedPrefix = "transformed"

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Registering the transform is what creates the callback server and hands its
		// address to the engine. On a topology where the engine cannot dial back, the
		// failure surfaces here or at the first resource registration below.
		err := ctx.RegisterResourceTransform(
			func(_ context.Context, args *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				if args.Type != "random:index/randomPet:RandomPet" {
					return nil
				}
				props := args.Props
				if props == nil {
					props = pulumi.Map{}
				}
				props["prefix"] = pulumi.String(transformedPrefix)
				return &pulumi.ResourceTransformResult{Props: props, Opts: args.Opts}
			})
		if err != nil {
			return err
		}

		pet, err := random.NewRandomPet(ctx, "pet", &random.RandomPetArgs{})
		if err != nil {
			return err
		}

		ctx.Log.Info("oci transform program registered a resource transform (engine must dial back)", nil)
		ctx.Export("petName", pet.ID())
		return nil
	})
}
