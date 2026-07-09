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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Pull pulls an image ref, streaming the runtime's progress output to output.
// Auth-shaped failures get a note: private registries are not yet supported.
func (r *Runtime) Pull(ctx context.Context, ref string, output io.Writer) error {
	cmd := exec.CommandContext(ctx, r.Path, "pull", ref)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pulling policy pack image %s failed: %w "+
			"(note: private registries are not yet supported for policy packs — "+
			"the image must be pullable anonymously)", ref, err)
	}
	return nil
}

// ResolveDigest returns the digest-pinned ref for a tagged ref that has
// already been pulled, e.g. "ghcr.io/acme/pack@sha256:…".
func (r *Runtime) ResolveDigest(ctx context.Context, ref string) (string, error) {
	out, err := r.run(ctx, "image", "inspect", "--format", "{{json .RepoDigests}}", ref)
	if err != nil {
		return "", fmt.Errorf("resolving digest for %s: %w", ref, err)
	}
	var digests []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &digests); err != nil {
		return "", fmt.Errorf("resolving digest for %s: unexpected inspect output %q: %w", ref, out, err)
	}
	repo := ref
	if i := strings.Index(repo, "@"); i >= 0 {
		repo = repo[:i]
	}
	if i := strings.LastIndex(repo, ":"); i > strings.LastIndex(repo, "/") {
		repo = repo[:i]
	}
	first := ""
	for _, d := range digests {
		if !strings.Contains(d, "@sha256:") {
			continue
		}
		if strings.HasPrefix(d, repo+"@") {
			return d, nil
		}
		if first == "" {
			first = d
		}
	}
	if first != "" {
		return first, nil
	}
	return "", fmt.Errorf("could not resolve a registry digest for %s: the image has no repository digest "+
		"(has it been pushed to its registry?)", ref)
}

// HasPlatform reports whether the (pulled) ref supports platform ("os/arch").
// Multi-arch refs are checked via `manifest inspect`; single-platform images
// fall back to the pulled image's os/architecture.
func (r *Runtime) HasPlatform(ctx context.Context, ref, platform string) (bool, error) {
	out, err := r.run(ctx, "manifest", "inspect", ref)
	if err == nil {
		var manifest struct {
			Manifests []struct {
				Platform struct {
					OS           string `json:"os"`
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if jsonErr := json.Unmarshal([]byte(out), &manifest); jsonErr == nil && len(manifest.Manifests) > 0 {
			for _, m := range manifest.Manifests {
				if m.Platform.OS+"/"+m.Platform.Architecture == platform {
					return true, nil
				}
			}
			return false, nil
		}
	}
	// Single-platform image (or a runtime without manifest support): inspect
	// the pulled image itself.
	out, err = r.run(ctx, "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", ref)
	if err != nil {
		return false, fmt.Errorf("checking platforms for %s: %w", ref, err)
	}
	return strings.TrimSpace(out) == platform, nil
}
