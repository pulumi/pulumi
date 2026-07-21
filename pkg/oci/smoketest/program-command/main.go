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
// `command` provider — which shells out to the local toolchain — to exercise three
// things a provider container gets in the OCI pod model, split so a failure
// localizes to one of them:
//
//  1. The program's ambient *toolchain*: it runs jq, a binary baked onto the
//     program image's PATH and present in no provider image. This is what
//     run-from-program-image supplies and what the shared workspace mount cannot —
//     a mount carries files, not a toolchain — so a provider running from its own
//     image would fail here with "jq: not found". This is the control that pins
//     where the provider ran.
//
//  2. The program's *workspace*: it reads a file baked at /workspace/marker. This
//     shows the provider sees the program's workspace, but does NOT discriminate
//     where it ran — /workspace is the shared volume the program image seeds, so
//     any provider mounting it reads the marker too (see Dockerfile.command).
//
//  3. The engine's projected *environment*: it reads a credential-like variable
//     (OCI_SMOKE_FAKE_CRED) that the engine has but the program image does not.
//     The only way it reaches the provider is the container host projecting the
//     engine's environment onto the provider container — the mechanism by which a
//     real cloud provider would receive its credentials.
package main

import (
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Write a file at runtime into the shared workspace. This is the discriminator
		// for live volume sharing: the file does not exist in the program image (only
		// /workspace/marker is baked in), so a provider container can only read it if
		// it mounts the SAME workspace volume the program writes to — not a copy, not
		// a second volume, and not the image's baked content.
		if err := os.WriteFile("/workspace/runtime-output", []byte("written-at-runtime"), 0o644); err != nil {
			return err
		}
		// Run a binary that exists only on the program image's PATH. -n takes no input,
		// so the output is exactly the constant — deterministic enough to assert on.
		// "jq: not found" here means the provider did not run from the program image.
		tool, err := local.NewCommand(ctx, "run-toolchain", &local.CommandArgs{
			Create: pulumi.String(`jq -rn '"toolchain-from-the-program-image"'`),
		})
		if err != nil {
			return err
		}
		ctx.Log.Info("oci smoke-test ran a command provider rooted in the program image", nil)
		ctx.Export("toolchain", tool.Stdout)

		read, err := local.NewCommand(ctx, "read-marker", &local.CommandArgs{
			Create: pulumi.String("cat /workspace/marker"),
		})
		if err != nil {
			return err
		}
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

		// Read the runtime-written file. This proves live volume sharing: the program
		// wrote it into /workspace at runtime, and the command provider — running in a
		// separate container — reads it from the same shared mount. If volumes were
		// per-container or re-seeded from the image, this file would not exist.
		runtimeFile, err := local.NewCommand(ctx, "read-runtime-output", &local.CommandArgs{
			Create: pulumi.String("cat /workspace/runtime-output"),
		})
		if err != nil {
			return err
		}
		ctx.Export("runtimeOutput", runtimeFile.Stdout)
		return nil
	})
}
