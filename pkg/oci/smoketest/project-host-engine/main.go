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

// The Mode 1 smoke program: an ordinary host-run Pulumi program whose one
// containerized dependency is the random provider, opted in by its oci:// pin.
// The pin is the per-package containerization switch — everything else about
// this program is stock Pulumi.
package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		pet, err := random.NewRandomPet(ctx, "pet", &random.RandomPetArgs{},
			// The oci:// pin: resolved verbatim by the host-engine container
			// host, so the provider runs as a container pulled through the
			// smoke's registry proxy. Everything unpinned takes the stock path.
			pulumi.PluginDownloadURL("oci://localhost:5005/pulumi/pulumi-provider-random:v4.21.0"))
		if err != nil {
			return err
		}
		ctx.Export("petName", pet.ID())
		return nil
	})
}
