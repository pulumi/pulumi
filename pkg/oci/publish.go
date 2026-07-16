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

package oci

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PublishPackageImage tags a package image (present in the local daemon) as
// destRef and pushes it — the registry half of `pulumi package publish` for an
// OCI destination. The caller has already verified the artifact by running it
// (the schema read is the conformance check) and resolved destRef from the
// package's reported identity plus the publisher org.
//
// This is deliberately a HOST-side operation, not a PodManager verb: publishing
// happens where registry credentials live — the pod inherits images, not
// credentials. It is also still coupled to the container runtime (docker
// tag/push against the daemon); the design's stated future is publish as a pure
// registry op (pushing a build-produced OCI layout, crane-style), which would
// remove this coupling along with the build's (see the alt-builder notes).
func PublishPackageImage(ctx context.Context, srcRef, destRef string) error {
	if _, err := dockerCmd(ctx, "tag", srcRef, destRef); err != nil {
		return err
	}
	_, err := dockerCmd(ctx, "push", destRef)
	return err
}

// dockerCmd runs `docker <args...>` and returns trimmed stdout, mirroring
// dockerPodManager.docker for operations that belong to no pod.
func dockerCmd(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
