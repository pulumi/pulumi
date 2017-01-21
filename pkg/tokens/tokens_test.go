// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokens(t *testing.T) {
	// Package tokens/names.
	p := "test/package"
	assert.False(t, IsName(p))
	assert.True(t, IsQName(p))
	pkg := NewPackageToken(PackageName(p))
	assert.Equal(t, p, pkg.Name().String())
	assert.Equal(t, p, pkg.String())

	// Module tokens/names.
	m := "my/module"
	assert.False(t, IsName(m))
	assert.True(t, IsQName(m))
	mod := NewModuleToken(pkg, ModuleName(m))
	assert.Equal(t, m, mod.Name().String())
	assert.Equal(t, p, mod.Package().Name().String())
	assert.Equal(t, p+ModuleDelimiter+m, mod.String())

	// Module member tokens/names.
	mm := "memby"
	assert.True(t, IsName(mm))
	assert.True(t, IsQName(mm))
	modm := NewModuleMemberToken(mod, ModuleMemberName(mm))
	assert.Equal(t, mm, modm.Name().String())
	assert.Equal(t, m, modm.Module().Name().String())
	assert.Equal(t, p, modm.Module().Package().Name().String())
	assert.Equal(t, p+ModuleDelimiter+m+ModuleMemberDelimiter+mm, modm.String())

	// Class member tokens/names.
	cm := "property"
	assert.True(t, IsName(cm))
	assert.True(t, IsQName(cm))
	clm := NewClassMemberToken(Type(modm), ClassMemberName(cm))
	assert.Equal(t, cm, clm.Name().String())
	assert.Equal(t, mm, clm.Class().Name().String())
	assert.Equal(t, m, clm.Class().Module().Name().String())
	assert.Equal(t, p, clm.Class().Module().Package().Name().String())
	assert.Equal(t, p+ModuleDelimiter+m+ModuleMemberDelimiter+mm+ClassMemberDelimiter+cm, clm.String())
}
