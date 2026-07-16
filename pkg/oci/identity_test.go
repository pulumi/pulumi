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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseProviderImageRef(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ref  string
		want PackageIdentity
		ok   bool
	}{
		// The org-published form the publish flow produces.
		{
			"localhost:5062/spikeorg/pulumi-provider-greeting:v0.1.0",
			PackageIdentity{Org: "spikeorg", Name: "greeting", Version: "0.1.0"}, true,
		},
		// A released first-party provider: the pulumi org is an org like any other.
		{
			"ghcr.io/pulumi/pulumi-provider-random:v4.21.0",
			PackageIdentity{Org: "pulumi", Name: "random", Version: "4.21.0"}, true,
		},
		// No registry host: still identity — org plus kind-prefixed leaf.
		{
			"spikeorg/pulumi-provider-greeting:v0.1.0",
			PackageIdentity{Org: "spikeorg", Name: "greeting", Version: "0.1.0"}, true,
		},
		// The '+'→'_' tag mapping reverses (semver never contains '_').
		{
			"localhost:5062/pulumi/pulumi-provider-dev:v0.1.0-alpha.0_dev",
			PackageIdentity{Org: "pulumi", Name: "dev", Version: "0.1.0-alpha.0+dev"}, true,
		},

		// Un-namespaced refs carry no identity: the grammar has exactly one
		// shape, and a single-segment repository is not a degenerate form of it.
		{"pulumi-provider-random:v4.21.0", PackageIdentity{}, false},
		{"localhost:5062/pulumi-provider-random:v4.21.0", PackageIdentity{}, false},
		// Wrong leaf prefix: not a provider image.
		{"ghcr.io/spikeorg/greeting:v0.1.0", PackageIdentity{}, false},
		// No v-tag (or no tag at all): versionless refs are not identity.
		{"ghcr.io/spikeorg/pulumi-provider-greeting:latest", PackageIdentity{}, false},
		{"ghcr.io/spikeorg/pulumi-provider-greeting", PackageIdentity{}, false},
		// Deeper repository paths don't fit the convention.
		{"ghcr.io/a/b/pulumi-provider-x:v1.0.0", PackageIdentity{}, false},
		// A second host-looking segment (e.g. nested mirrors) is not an org.
		{"mirror.example.com/upstream.io:443/pulumi-provider-x:v1.0.0", PackageIdentity{}, false},
		// Empty name after the prefix.
		{"ghcr.io/spikeorg/pulumi-provider-:v1.0.0", PackageIdentity{}, false},
	}
	for _, c := range cases {
		got, ok := ParseProviderImageRef(c.ref)
		assert.Equal(t, c.ok, ok, "ok for %q", c.ref)
		assert.Equal(t, c.want, got, "identity for %q", c.ref)
	}
}

// The parse inverts the render: identity → ref → identity round-trips.
func TestProviderImageRefRoundTrip(t *testing.T) {
	t.Parallel()
	for _, id := range []PackageIdentity{
		{Org: "spikeorg", Name: "greeting", Version: "0.1.0"},
		{Org: "pulumi", Name: "random", Version: "4.21.0"},
		{Org: "pulumi", Name: "dev", Version: "0.1.0-alpha.0+dev"},
	} {
		ref := ProviderImageRefInOrg("reg.example.com:5000", id.Org, id.Name, id.Version)
		got, ok := ParseProviderImageRef(ref)
		assert.True(t, ok, "round-trip parse of %q", ref)
		assert.Equal(t, id, got, "round-trip identity of %q", ref)
	}
}

// An unpinned package renders under the default org — the same defaulting the
// registry applies to a bare package name, at the same layer (name resolution).
func TestProviderImageRefDefaultsOrg(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		"reg.example.com/pulumi/pulumi-provider-random:v4.21.0",
		ProviderImageRef("reg.example.com", "random", "4.21.0"))
	assert.Equal(t,
		"pulumi/pulumi-provider-random:v4.21.0",
		ProviderImageRef("", "random", "4.21.0"))
}

// The policy-pack convention shares the grammar with a different kind prefix, and
// the prefixes do not cross-parse: a provider ref carries no policy identity and
// vice versa.
func TestPolicyImageRefRoundTrip(t *testing.T) {
	t.Parallel()
	ref := PolicyImageRefInOrg("reg.example.com", "acme", "compliance", "1.2.3")
	assert.Equal(t, "reg.example.com/acme/pulumi-policy-compliance:v1.2.3", ref)

	id, ok := ParsePolicyImageRef(ref)
	assert.True(t, ok)
	assert.Equal(t, PackageIdentity{Org: "acme", Name: "compliance", Version: "1.2.3"}, id)

	_, ok = ParsePolicyImageRef("reg.example.com/acme/pulumi-provider-compliance:v1.2.3")
	assert.False(t, ok, "a provider ref is not policy identity")
	_, ok = ParseProviderImageRef(ref)
	assert.False(t, ok, "a policy ref is not provider identity")

	assert.Equal(t, "pulumi/pulumi-policy-compliance:v1.2.3",
		PolicyImageRef("", "compliance", "1.2.3"), "unpinned packs render under the default org")
}
