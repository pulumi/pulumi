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

// A Pulumi program for the CRI multi-source consume proof. It creates one resource
// through each of TWO explicit `random` providers that resolve to DIFFERENT sources:
//
//   - pub:  an unpinned first-party provider. With no oci:// pin it resolves by
//           convention under the constant public source (pulumi.registry.internal),
//           which the proxy synthesizes.
//   - priv: the SAME package (pulumi/pulumi-provider-random) pinned to a PRIVATE
//           source. Its oci:// ref names its own host, so it resolves there verbatim.
//
// Same publisher, same name, different SOURCE — the exact case a single registry knob
// could never express (one knob = one host). Explicit providers are keyed by URN, not
// package name, so each carries its own PluginDownloadURL into the engine's provider
// descriptor and the container host resolves each to its own image. A green run with
// both pets, each pulled from its own registry host, is the proof.
package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The first-party provider: no pin, resolves under the public source.
		pub, err := random.NewProvider(ctx, "pub", &random.ProviderArgs{})
		if err != nil {
			return err
		}
		// The private provider: same package, pinned to a private source by its oci:// ref.
		priv, err := random.NewProvider(ctx, "priv", &random.ProviderArgs{},
			pulumi.PluginDownloadURL("oci://private.registry.internal/pulumi/pulumi-provider-random:v4.21.0"))
		if err != nil {
			return err
		}

		petPub, err := random.NewRandomPet(ctx, "pet-pub", &random.RandomPetArgs{}, pulumi.Provider(pub))
		if err != nil {
			return err
		}
		petPriv, err := random.NewRandomPet(ctx, "pet-priv", &random.RandomPetArgs{}, pulumi.Provider(priv))
		if err != nil {
			return err
		}

		ctx.Log.Info("oci multi-source program created a resource via a first-party AND a private provider", nil)
		ctx.Export("petPub", petPub.ID())
		ctx.Export("petPriv", petPriv.ID())
		return nil
	})
}
