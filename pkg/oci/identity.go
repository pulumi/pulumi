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

import "strings"

// DefaultPackageOrg is the org under which a package with no pin resolves —
// released first-party providers. This mirrors how the registry resolves a bare
// package name today: bare "random" is name-resolution sugar for the fully
// qualified pulumi/pulumi/random (see sdk/go/common/registry/resolve.go), and
// the default is applied at resolution time, never stored in anything durable.
// There is no un-namespaced ref form: every image ref carries an org segment
// (compare Docker Hub, where bare "ubuntu" is client sugar for
// "library/ubuntu" and the wire always sees both segments).
const DefaultPackageOrg = "pulumi"

// PackageIdentity is a package's identity: the publishing org, the package
// name, and a version. A convention-shaped image ref carries its identity
// (ParseProviderImageRef recovers it); the location — which registry serves
// the image — is chosen at resolution time: the registry knob when set, else
// the ref's own host.
type PackageIdentity struct {
	Org     string // publishing org ("pulumi" for released first-party providers)
	Name    string // package name ("greeting", "random")
	Version string // semver, no leading "v"
}

// ParseProviderImageRef reduces a convention-shaped provider image ref —
// [registry/]<org>/pulumi-provider-<name>:v<version> — to the identity it
// encodes. ok is false when the ref does not follow the convention (no org
// segment, wrong repo prefix, no v-tag, deeper repository paths); such a ref
// carries no identity and callers must treat it as an opaque location — usable
// verbatim, but not relocatable by the registry knob.
//
// The grammar has exactly one shape: an org segment, then the kind-prefixed
// leaf. A single-segment repository is NOT a degenerate convention ref —
// un-namespaced identities do not exist (see DefaultPackageOrg).
//
// The leading segment is taken as a registry host by the docker heuristic (it
// contains "." or ":", or is "localhost") and discarded: identity carries no
// location. The tag's '+'→'_' mapping from ProviderImageRefInOrg is reversed,
// which is unambiguous because semver versions never contain '_'.
func ParseProviderImageRef(ref string) (PackageIdentity, bool) {
	repo, tag := ref, ""
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		repo, tag = ref[:i], ref[i+1:]
	}
	if !strings.HasPrefix(tag, "v") || len(tag) < 2 {
		return PackageIdentity{}, false
	}
	version := strings.ReplaceAll(strings.TrimPrefix(tag, "v"), "_", "+")

	segs := strings.Split(repo, "/")
	if len(segs) > 1 && (strings.ContainsAny(segs[0], ".:") || segs[0] == "localhost") {
		segs = segs[1:] // a registry host is location, not identity
	}
	if len(segs) != 2 {
		return PackageIdentity{}, false
	}
	org := segs[0]
	if org == "" || strings.ContainsAny(org, ".:") {
		// A second host-looking segment (e.g. nested mirrors) is not an org.
		return PackageIdentity{}, false
	}
	name := strings.TrimPrefix(segs[1], "pulumi-provider-")
	if name == "" || name == segs[1] {
		return PackageIdentity{}, false
	}
	return PackageIdentity{Org: org, Name: name, Version: version}, true
}
