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

// A Pulumi program for the containerized-provider smoke test. Unlike the trivial
// program/ (which registers no resources), this one creates a real resource via
// the `random` provider, so a successful run proves the engine drove a *provider
// running in its own container* through the full RegisterResource → Create gRPC
// path. random needs no cloud credentials and generates its value locally.
package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		pet, err := random.NewRandomPet(ctx, "smoke-pet", &random.RandomPetArgs{})
		if err != nil {
			return err
		}
		ctx.Log.Info("oci smoke-test program created a random resource via a containerized provider", nil)
		ctx.Export("petName", pet.ID())
		return nil
	})
}
