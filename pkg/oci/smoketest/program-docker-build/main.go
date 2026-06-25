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

// A Pulumi program for the image-build smoke test — the workspace-coupled
// provider "real prize". It builds a container image with the `docker` provider
// from a build context (/workspace/app) baked into the program image. A
// successful run proves two things at once: the docker provider ran *rooted in
// the program filesystem* (run-from-program-image), so it resolved the build
// context (and used the docker CLI) from paths that exist only in the program
// image; and it reached the docker daemon through the projected docker socket
// (the capability mechanism).
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
