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

func TestTokens(t *testing.T) {
	t.Parallel()

	// Package tokens/names.
	p := "test/package"
	assert.False(t, IsName(p))
	assert.True(t, IsQName(p))
	pkg := NewPackageToken(PackageName(p))
	assert.Equal(t, p, pkg.Name().String())
	assert.Equal(t, p, pkg.String())
	p2 := "test/my-package"
	assert.False(t, IsName(p2))
	assert.True(t, IsQName(p2))
	pkg2 := NewPackageToken(PackageName(p2))
	assert.Equal(t, p2, pkg2.Name().String())
	assert.Equal(t, p2, pkg2.String())

	// Module tokens/names.
	m := "my/module"
	assert.False(t, IsName(m))
	assert.True(t, IsQName(m))
	mod := NewModuleToken(pkg, ModuleName(m))
	assert.Equal(t, m, mod.Name().String())
	assert.Equal(t, p, mod.Package().Name().String())
	assert.Equal(t, p+TokenDelimiter+m, mod.String())

	// Module member tokens/names.
	mm := "memby"
	assert.True(t, IsName(mm))
	assert.True(t, IsQName(mm))
	modm := NewModuleMemberToken(mod, ModuleMemberName(mm))
	assert.Equal(t, mm, modm.Name().String())
	assert.Equal(t, m, modm.Module().Name().String())
	assert.Equal(t, p, modm.Module().Package().Name().String())
	assert.Equal(t, p+TokenDelimiter+m+TokenDelimiter+mm, modm.String())
}

func TestTypeDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give Type
		want string
	}{
		{
			desc: "not enough parts",
			give: "incomplete",
			want: "incomplete",
		},
		{
			desc: "no name",
			give: "pkg:mod:",
			want: "pkg:mod:",
		},
		{
			desc: "no slash",
			give: "pkg:mod:typ",
			want: "pkg:mod:typ",
		},
		{
			desc: "bad casing",
			give: "pkg:Mod/foo:typ",
			want: "pkg:Mod/foo:typ",
		},
		{
			desc: "remove slash",
			give: "pkg:mod/foo/bar:Bar",
			want: "pkg:mod/foo:Bar",
		},
		{
			desc: "remove up to last slash",
			give: "pkg:mod/foo/bar/baz:Baz",
			want: "pkg:mod/foo/bar:Baz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.give.DisplayName())
		})
	}
}
