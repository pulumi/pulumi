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

// A Node Pulumi program for the image-build smoke test's buildkit half, driven
// from a *Node* host to sidestep the docker-build Go SDK's module-consumability
// snag. It builds a container image with @pulumi/docker-build from a context
// (/workspace/app) baked into the program image.
//
// Unlike the classic `docker` provider (which shells out to `docker build`),
// docker-build uses an *embedded buildkit client* and talks to the daemon
// directly — so no docker CLI is needed anywhere: not in the program image, which
// carries only the build context, and not in the provider's own image either.
// A successful run proves the docker-build provider — running from its own image,
// as every provider but `command` does — resolved the build context through the
// shared /workspace mount that the program image seeds, and reached the daemon
// through the projected docker socket (the capability mechanism), producing a
// real, inspectable image.
"use strict";

const pulumi = require("@pulumi/pulumi");
const dockerBuild = require("@pulumi/docker-build");

const image = new dockerBuild.Image("demo", {
    context: { location: "/workspace/app" },
    dockerfile: { location: "/workspace/app/Dockerfile" },
    tags: ["oci-pod-buildx-built:latest"],
    // Explicitly export the build result into the (projected) docker daemon's
    // image store. Without an export, buildkit builds but emits no image and an
    // empty digest ("No exports were specified") — `load: true` is what makes a
    // real, inspectable artifact with a populated digest, rather than a cache-only
    // build we could only assert on indirectly (e.g. contextHash).
    push: false,
    load: true,
});

pulumi.log.info(
    "oci smoke-test: built an image via the docker-build (buildkit) provider from the program workspace");

exports.ref = image.ref;
exports.digest = image.digest;
exports.contextHash = image.contextHash;
