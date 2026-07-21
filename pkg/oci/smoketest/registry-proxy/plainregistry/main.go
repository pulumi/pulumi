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

// Command plainregistry is a bare, read-write, in-memory OCI registry — ggcr's
// pkg/registry with nothing bolted on. It is the ImportImage SINK for the CRI
// build+publish smoke test: `pulumi package build` pushes the built layout here
// (ggcr remote.Write, anonymous, plain-HTTP) and containerd pulls it back into the
// k8s.io namespace via a plain-http hosts.toml.
//
// Why not reuse the sibling registry-proxy? That proxy reserves the provider
// namespace (pulumi/pulumi-provider-*) as READ-ONLY — it synthesizes those images
// from released binaries and 405s any push into them. The build sink's ref is
// unavoidably ProviderImageRef -> pulumi/pulumi-provider-<name>, exactly that
// namespace, so a push through the proxy is rejected. That rejection is a genuine
// finding (a synthesizing proxy cannot double as the build sink for the provider
// namespace without a local-build-shadows-synthesis precedence rule); this plain
// registry sidesteps it so the CRI build+publish mechanism can be proven on its own.
//
// Bootstrap/testing only: no auth, no TLS, in-memory store dies with the process.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/registry"
)

func main() {
	addr := os.Getenv("PLAIN_REGISTRY_ADDR")
	if addr == "" {
		addr = ":5000"
	}
	log.Printf("plainregistry: listening on %s (read-write in-memory OCI registry)", addr)
	log.Fatal(http.ListenAndServe(addr, registry.New()))
}
