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

// A Go Pulumi program that consumes a Node multi-language component — the payoff
// of the prototype. The program runs as one pod container; when it creates a
// greeting:index:Greeter, the engine starts the Node component as another pod
// container and calls Construct on it. A Go program driving a Node component,
// both as containers on the pod, is the program=component unification made real.
//
// No generated SDK for the component is needed: RegisterRemoteComponentResource
// drives it by its type token, and the version is pinned so the container host
// resolves pulumi-provider-greeting:v0.1.0 by convention.
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Greeter mirrors the component's outputs; the `pulumi:"message"` tag binds the
// Construct response's state to this field.
type Greeter struct {
	pulumi.ResourceState
	Message pulumi.StringOutput `pulumi:"message"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var g Greeter
		err := ctx.RegisterRemoteComponentResource(
			"greeting:index:Greeter", "smoke-greeter",
			pulumi.Map{"who": pulumi.String("the pod")},
			&g,
			pulumi.Version("0.1.0"),
		)
		if err != nil {
			return err
		}
		ctx.Log.Info("oci smoke-test: Go program created a Node multi-language component", nil)
		ctx.Export("message", g.Message)
		return nil
	})
}
