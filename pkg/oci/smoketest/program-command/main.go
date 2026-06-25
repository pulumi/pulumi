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

// A Pulumi program for the workspace-coupled-provider smoke test. It uses the
// `command` provider — which shells out to the local toolchain — to read a file
// baked into the program image's own workspace (/workspace/marker). A successful
// run proves the command provider ran *rooted in the program's filesystem*
// (design's run-from-program-image model): it saw a file that exists only in the
// program image, not in the provider image, and not via a copied volume. This is
// the case a read-only workspace volume could never serve, because `command`
// needs the program's toolchain, not just its files.
package main

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		read, err := local.NewCommand(ctx, "read-marker", &local.CommandArgs{
			Create: pulumi.String("cat /workspace/marker"),
		})
		if err != nil {
			return err
		}
		ctx.Log.Info("oci smoke-test ran a command provider rooted in the program image", nil)
		ctx.Export("marker", read.Stdout)
		return nil
	})
}
