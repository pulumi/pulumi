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

// The consumer program for the publish smoke test. Unlike the mlc consumer (which
// drives the component by raw type token), this program uses the SDK that
// `pulumi package add oci://<org ref>` GENERATED — that is the point: the generated
// SDK carries the oci:// ref as its PluginDownloadURL default, so creating a Greeter
// registers the remote component with its self-locating pin attached. The engine
// resolves the pin through the layered resolver — the registry knob overrides the
// pin's host when set, the pin's own host is the zero-config route otherwise. The
// component's RandomPet child forces the released random provider to resolve by
// convention under the default org — both through one router endpoint.
package main

import (
	"example.com/pulumi-greeting/sdk/go/greeting"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		g, err := greeting.NewGreeter(ctx, "refs-greeter", &greeting.GreeterArgs{
			Who: pulumi.String("the pinned org ref"),
		})
		if err != nil {
			return err
		}
		ctx.Export("message", g.Message)
		return nil
	})
}
