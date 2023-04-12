// Copyright 2016-2018, Pulumi Corporation.
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

package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAsName(t *testing.T) {
	t.Parallel()

	goodNames := []string{
		"simple",       // all alpha.
		"SiMplE",       // mixed-case alpha.
		"simple0",      // alphanumeric.
		"SiMpLe0",      // mixed-case alphanumeric.
		"_",            // permit underscore.
		"s1MPl3_",      // mixed-case alphanumeric/underscore.
		"_s1MPl3",      // ditto.
		"hy-phy",       // permit hyphens.
		".dotstart",    // start with .
		"-hyphenstart", // start with -
		"0num",         // start with numbers
		"9num",         // start with numbers
	}
	for _, nm := range goodNames {
		assert.True(t, IsName(nm), "IsName expected to be true: %v", nm)
	}

	goodQNames := []string{
		"namespace/complex",                   // multi-part name.
		"_naMeSpace0/coMpl3x32",               // multi-part, alphanumeric, etc. name.
		"n_ameSpace3/moRenam3sp4ce/_Complex5", // even more complex parts.
	}
	for _, nm := range goodQNames {
		assert.True(t, IsQName(nm), "IsQName expected to be true: %v", nm)
		assert.False(t, IsName(nm), "IsName expected to be false: %v", nm)
	}

	badNames := []string{
		"s!mple",                          // bad characters.
		"namesp@ce/complex",               // ditto.
		"namespace/morenamespace/compl#x", // ditto.
	}
	for _, nm := range badNames {
		assert.False(t, IsName(nm), "IsName expected to be false: %v", nm)
		assert.False(t, IsQName(nm), "IsQName expected to be false: %v", nm)
	}
}

func TestNameSimple(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "simple", string(Name("simple")))
	assert.Equal(t, "complex", string(QName("namespace/complex").Name()))
	assert.Equal(t, "complex", string(QName("ns1/ns2/ns3/ns4/complex").Name()))
	assert.Equal(t, "c0Mpl3x_", string(QName("_/_/_/_/a0/c0Mpl3x_").Name()))
}

func TestNameNamespace(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "namespace", string(QName("namespace/complex").Namespace()))
	assert.Equal(t, "ns1/ns2/ns3/ns4", string(QName("ns1/ns2/ns3/ns4/complex").Namespace()))
	assert.Equal(t, "_/_/_/_/a0", string(QName("_/_/_/_/a0/c0Mpl3x_").Namespace()))
}

func TestIntoQName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"foo/bar", "foo/bar"},
		{input: "https:", expected: "https_"},
		{
			"https://github.com/pulumi/pulumi/blob/master/pkg/resource/deploy/providers/provider.go#L61-L86",
			"https_/github.com/pulumi/pulumi/blob/master/pkg/resource/deploy/providers/provider.go_L61-L86",
		},
		{"", "_"},
		{"///", "_"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, QName(c.expected), IntoQName(c.input))
		})
	}
}
