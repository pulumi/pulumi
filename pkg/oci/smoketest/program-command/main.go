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

// A Pulumi program for the run-from-program-image provider smoke test. It uses the
// `command` provider — which shells out to the local toolchain — to exercise two
// things a provider container gets in the OCI pod model:
//
//  1. The program's own *filesystem*: it reads a file baked into the program
//     image's workspace (/workspace/marker). Note this shows only that the
//     provider sees the program's workspace, not that it ran rooted in the
//     program's filesystem — /workspace is the shared volume the program image
//     seeds, so a provider running from its own image reads it too. The program's
//     ambient toolchain is what run-from-program-image actually supplies, and this
//     test does not yet isolate it (see Dockerfile.command).
//  2. The engine's projected *environment*: it reads a credential-like variable
//     (OCI_SMOKE_FAKE_CRED) that the engine has but the program image does not.
//     The only way it reaches the provider is the container host projecting the
//     engine's environment onto the provider container — the mechanism by which a
//     real cloud provider would receive its credentials.
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

		// Read a credential projected from the engine's environment. printf (no
		// newline) so the output is exactly the value; if projection did not happen
		// the variable is unset and stdout is empty.
		cred, err := local.NewCommand(ctx, "read-cred", &local.CommandArgs{
			Create: pulumi.String(`printf '%s' "$OCI_SMOKE_FAKE_CRED"`),
		})
		if err != nil {
			return err
		}
		ctx.Export("cred", cred.Stdout)
		return nil
	})
}
