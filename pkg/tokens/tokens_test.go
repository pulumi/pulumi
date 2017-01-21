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
	pkg := NewPackage(PackageName(p))
	assert.Equal(t, p, string(pkg.Name()))
	assert.Equal(t, p, string(pkg))

	// Module tokens/names.
	m := "my/module"
	assert.False(t, IsName(m))
	assert.True(t, IsQName(m))
	mod := NewModule(pkg, ModuleName(m))
	assert.Equal(t, m, string(mod.Name()))
	assert.Equal(t, p, string(mod.Package().Name()))
	assert.Equal(t, p+ModuleDelimiter+m, string(mod))

	// Module member tokens/names.
	mm := "memby"
	assert.True(t, IsName(mm))
	assert.True(t, IsQName(mm))
	modm := NewModuleMember(mod, ModuleMemberName(mm))
	assert.Equal(t, mm, string(modm.Name()))
	assert.Equal(t, m, string(modm.Module().Name()))
	assert.Equal(t, p, string(modm.Module().Package().Name()))
	assert.Equal(t, p+ModuleDelimiter+m+ModuleMemberDelimiter+mm, string(modm))

	// Class member tokens/names.
	cm := "property"
	assert.True(t, IsName(cm))
	assert.True(t, IsQName(cm))
	clm := NewClassMember(Type(modm), ClassMemberName(cm))
	assert.Equal(t, cm, string(clm.Name()))
	assert.Equal(t, mm, string(clm.Class().Name()))
	assert.Equal(t, m, string(clm.Class().Module().Name()))
	assert.Equal(t, p, string(clm.Class().Module().Package().Name()))
	assert.Equal(t, p+ModuleDelimiter+m+ModuleMemberDelimiter+mm+ClassMemberDelimiter+cm, string(clm))
}
