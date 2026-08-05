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

package engine

// TEMPORARY, non-shipping dogfooding hook for the OCI containerized-execution prototype.
//
// Org-mandatory ("required") policy packs are resolved by the backend and handed to the
// engine as downloadable tarballs (npm/python), then run through the language-host spawn
// path. That path never touches the OCI host's image resolution, and `--policy-pack` only
// *adds* packs — it cannot *replace* a required one — so a `runtime: oci` engine has no way
// to run a required pack as a container: the tarball still downloads and installs (and fails
// in a pod with no language toolchain). That inability to override your org's required
// policies is a *deliberate* product gap, which is exactly why this lives behind an env var,
// in its own file, and is wired into no shipping surface.
//
// PULUMI_OCI_REQUIRED_POLICY_IMAGES lets a test operator replace required policy packs with
// OCI images so the pod can be stress-tested against real internal stacks:
//
//	PULUMI_OCI_REQUIRED_POLICY_IMAGES="pulumi-internal-policies=oci://ghcr.io/acme/pulumi-policy-internal:v1"
//
// Entries are `name=ref` (an optional `@version` on the name is tolerated and ignored — the
// match is by name), comma- or semicolon-separated; a bare ref is prefixed with `oci://`. A
// matching required pack is redirected to its image and its tarball is neither downloaded nor
// installed. This is override-only, on purpose: to *add* an OCI policy pack (against a local
// backend that returns no required packs, say) use `--policy-pack oci://<ref>`, which already
// routes through the OCI host — this hook exists solely for the replace case that flag can't do.
//
// The env var is a no-op when unset. Delete this file to remove the hook.

import (
	"fmt"
	"os"
	"strings"
)

const requiredPolicyImagesEnvVar = "PULUMI_OCI_REQUIRED_POLICY_IMAGES"

// policyImageOverride is one parsed `name=ref` mapping.
type policyImageOverride struct {
	name string
	ref  string // always oci://-prefixed
}

// parseRequiredPolicyImageOverrides parses the env var value into ordered mappings.
// Malformed entries are reported and skipped rather than failing the update — this is a
// test hook, not a validated input surface.
func parseRequiredPolicyImageOverrides(raw string) []policyImageOverride {
	var out []policyImageOverride
	for _, entry := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' }) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		left, ref, found := strings.Cut(entry, "=")
		left, ref = strings.TrimSpace(left), strings.TrimSpace(ref)
		if !found || left == "" || ref == "" {
			fmt.Fprintf(os.Stderr, "oci: ignoring malformed %s entry %q (want name=ref)\n",
				requiredPolicyImagesEnvVar, entry)
			continue
		}
		name, _, _ := strings.Cut(left, "@") // tolerate name@version; match by name
		if !strings.HasPrefix(ref, "oci://") {
			ref = "oci://" + ref
		}
		out = append(out, policyImageOverride{name: name, ref: ref})
	}
	return out
}

// applyRequiredPolicyImageOverrides replaces backend-provided required policy packs named by
// PULUMI_OCI_REQUIRED_POLICY_IMAGES with OCI images. Returns the input unchanged when the env
// var is unset or empty. A mapping that names no required pack is reported (with a nudge to
// --policy-pack for the add case) but is otherwise inert.
func applyRequiredPolicyImageOverrides(policies []RequiredPolicy) []RequiredPolicy {
	overrides := parseRequiredPolicyImageOverrides(os.Getenv(requiredPolicyImagesEnvVar))
	if len(overrides) == 0 {
		return policies
	}

	byName := make(map[string]string, len(overrides))
	for _, o := range overrides {
		byName[o.name] = o.ref
	}

	matched := make(map[string]bool, len(overrides))
	result := make([]RequiredPolicy, len(policies))
	for i, p := range policies {
		if ref, ok := byName[p.Name()]; ok {
			matched[p.Name()] = true
			fmt.Fprintf(os.Stderr, "oci: required policy pack %q overridden to image %s\n", p.Name(), ref)
			result[i] = ociImageRequiredPolicy{RequiredPolicy: p, ref: ref}
			continue
		}
		result[i] = p
	}
	for _, o := range overrides {
		if !matched[o.name] {
			fmt.Fprintf(os.Stderr,
				"oci: %s names %q, but the backend returned no required policy pack by that name; "+
					"to ADD an OCI policy pack (rather than override a required one) use --policy-pack oci://<ref>\n",
				requiredPolicyImagesEnvVar, o.name)
		}
	}
	return result
}

// ociImageRequiredPolicy decorates a backend-provided RequiredPolicy so it resolves to an
// OCI image. Installed()==true short-circuits installPolicyPack (no download/install), and
// LocalPath() hands the OCI host an oci:// ref its PolicyAnalyzer runs as a pinned image;
// everything else delegates to the wrapped policy so config and naming survive.
type ociImageRequiredPolicy struct {
	RequiredPolicy
	ref string
}

func (p ociImageRequiredPolicy) Installed() bool            { return true }
func (p ociImageRequiredPolicy) LocalPath() (string, error) { return p.ref, nil }
