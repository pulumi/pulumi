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

// A Pulumi program for the image-build smoke test — the "real prize" for a provider
// that needs things from outside its own image. It builds a container image with the
// `docker` provider from a build context (/workspace/app) baked into the program
// image. A successful run proves two things at once: the docker provider resolved a
// build context that exists only because the *program's* image seeded the shared
// workspace, reaching it through the mount every provider gets; and it reached the
// docker daemon through the projected docker socket (the capability mechanism).
//
// The provider runs from its OWN image, which carries its docker CLI (see
// Dockerfile.docker-provider) — that CLI is the provider's toolchain, not the
// program's. Whether that is the right call is still open (#56): running it from the
// program image, as `command` does, remains arguable.
//
// The classic `docker` provider is used rather than `docker-build` only because
// the latter's Go SDK is not cleanly consumable via go modules right now; the pod
// execution model is identical for both.
package main

import (
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		img, err := docker.NewImage(ctx, "demo", &docker.ImageArgs{
			Build: &docker.DockerBuildArgs{
				Context:    pulumi.String("/workspace/app"),
				Dockerfile: pulumi.String("/workspace/app/Dockerfile"),
			},
			ImageName: pulumi.String("oci-pod-built:latest"),
			SkipPush:  pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Log.Info("oci smoke-test built an image via the docker provider from the program workspace", nil)
		ctx.Export("imageName", img.ImageName)
		return nil
	})
}
