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

// The Mode 1 MLC smoke program: registers the containerized greeting component
// remotely, hand-rolled (no generated SDK — RegisterRemoteComponentResource is
// exactly what a generated SDK would emit). The oci:// pin is the per-package
// opt-in; the component's Construct runs in its container and dials the monitor
// back at host.docker.internal to register itself and its RandomPet child.
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Greeter mirrors the component's schema: one output, message, which embeds the
// child RandomPet's generated name — so a populated message is a receipt that
// Construct ran, dialed back, and built real infrastructure.
type Greeter struct {
	pulumi.ResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		g := &Greeter{}
		err := ctx.RegisterRemoteComponentResource(
			"greeting:index:Greeter", "hello",
			pulumi.Map{"who": pulumi.String("claire")}, g,
			pulumi.Version("0.1.0"),
			// The pin: the image is tagged locally under this ref, so the host
			// engine's store check hits without any registry running.
			pulumi.PluginDownloadURL("oci://localhost:5005/pulumi/pulumi-provider-greeting:v0.1.0"))
		if err != nil {
			return err
		}
		ctx.Export("message", g.Message)
		return nil
	})
}
