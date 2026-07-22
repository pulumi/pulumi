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
	"os"
	"strings"
)

// DefaultPackageOrg is the org under which a package with no pin resolves —
// released first-party providers. This mirrors how the registry resolves a bare
// package name today: bare "random" is name-resolution sugar for the fully
// qualified pulumi/pulumi/random (see sdk/go/common/registry/resolve.go), and
// the default is applied at resolution time, never stored in anything durable.
// There is no un-namespaced ref form: every image ref carries an org segment
// (compare Docker Hub, where bare "ubuntu" is client sugar for
// "library/ubuntu" and the wire always sees both segments).
const DefaultPackageOrg = "pulumi"

// DefaultPublicRegistry is the fallback host for the PUBLIC source — where an
// unpinned package (a released first-party provider, or a component consumed
// without a pin) resolves by convention, exactly as bare "ubuntu" resolves under
// docker.io. It is only the fallback: PublicRegistry() lets the pod override it
// with PULUMI_POD_PUBLIC_REGISTRY, and the bootstrap wrapper points it at a
// routable registry. A pinned package names its own host and is NEVER rewritten
// to this one — the public host qualifies only unpinned convention refs, never a
// pin — which is what lets a first-party package and a private one resolve to
// their own sources side by side.
//
// The fallback is a clearly-fake simulation host (the prototype has no real
// public registry yet); it flips to a real host in one line the day one exists,
// with the ref grammar unchanged so pins and convention refs keep working.
// Reachability is an address-layer concern, not an identity one, so no port
// belongs here — the routable override the wrapper supplies carries the port.
const DefaultPublicRegistry = "pulumi.registry.internal"

// DefaultPrivateRegistry is the fallback host for the PRIVATE source — where a
// local component build is tagged, and the default target that `package build`
// and `package publish` send their output to. PrivateRegistry() lets the pod
// override it with PULUMI_POD_PRIVATE_REGISTRY; the bootstrap wrapper points it
// at a routable registry. It is deliberately distinct from DefaultPublicRegistry:
// the public and private sources are separate hosts, and keeping the build's tag
// host separate from the public resolve host is the whole point — conflating them
// is the bug this split fixes. A consumer pins a built package to this host and
// the engine resolves that pin verbatim.
const DefaultPrivateRegistry = "private.registry.internal"

// PublicRegistry returns the public source host: where unpinned convention refs
// resolve. It is read at resolve time by the container host.
// PULUMI_POD_PUBLIC_REGISTRY overrides DefaultPublicRegistry. It only qualifies
// an unpinned ref — a pin is always used verbatim — so setting it can never
// relocate a package that names its own source.
func PublicRegistry() string {
	if r := os.Getenv("PULUMI_POD_PUBLIC_REGISTRY"); r != "" {
		return r
	}
	return DefaultPublicRegistry
}

// PrivateRegistry returns the private source host: the default host `package
// build`/`publish` tag their output with, and the host the language host tags a
// local component build under. PULUMI_POD_PRIVATE_REGISTRY overrides
// DefaultPrivateRegistry. Like the public host it only stamps a host onto a ref
// being created; it never rewrites an existing pin.
func PrivateRegistry() string {
	if r := os.Getenv("PULUMI_POD_PRIVATE_REGISTRY"); r != "" {
		return r
	}
	return DefaultPrivateRegistry
}

// PackageIdentity is a package's identity: the publishing org, the package
// name, and a version. A convention-shaped image ref carries its identity
// (ParseProviderImageRef recovers it); the location — which registry serves the
// image — is separate: a pin names its own host and is used verbatim, while an
// unpinned convention ref is qualified with the public source at resolve time.
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
// verbatim, but never re-qualified under the public source.
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
	return parsePackageImageRef(ref, providerRepoPrefix)
}

// ParsePolicyImageRef is ParseProviderImageRef for the policy-pack convention
// ([registry/]<org>/pulumi-policy-<name>:v<version>).
func ParsePolicyImageRef(ref string) (PackageIdentity, bool) {
	return parsePackageImageRef(ref, policyRepoPrefix)
}

func parsePackageImageRef(ref, kindPrefix string) (PackageIdentity, bool) {
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
	name := strings.TrimPrefix(segs[1], kindPrefix)
	if name == "" || name == segs[1] {
		return PackageIdentity{}, false
	}
	return PackageIdentity{Org: org, Name: name, Version: version}, true
}
