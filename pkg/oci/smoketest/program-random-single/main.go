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

// A minimal single-provider program for the address-model mechanism smoke test. It creates
// one RandomPet through the DEFAULT `random` provider, which the container host resolves by
// convention to the local pulumi/pulumi-provider-random image. One provider, one image — no
// multi-source registry moving parts — so a green run isolates the forwarder-shim + attach-
// by-DNS path.
package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		pet, err := random.NewRandomPet(ctx, "pet", &random.RandomPetArgs{})
		if err != nil {
			return err
		}
		ctx.Log.Info("oci address-model program created a resource via a containerized provider", nil)
		ctx.Export("petName", pet.ID())
		return nil
	})
}
