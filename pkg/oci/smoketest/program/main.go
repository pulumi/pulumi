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

// A minimal Pulumi program for the OCI containerized-execution smoke test. It
// registers no resources (so no provider is needed) — it just connects to the
// resource monitor and exports stack outputs, which is enough to prove the
// program reached the engine. The hostname output makes it visible whether the
// program ran as a host subprocess or inside a container.
package main

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hostname, _ := os.Hostname()
		ctx.Log.Info("oci smoke-test program connected to the engine", nil)
		ctx.Export("greeting", pulumi.String("hello from a Pulumi program executed via the OCI runtime"))
		ctx.Export("hostname", pulumi.String(hostname))
		return nil
	})
}
