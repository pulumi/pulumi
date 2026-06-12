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

package rapidschema

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionRoundTrips(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		v := Version().Draw(t, "v")
		assert.Equal(t, v, semver.MustParse(v.String()))
	})
}

// TestPackageBindsMarshalsAndRoundTrips validates that a generated package is
// round-tripable through a schema spec.
//
// Since all valid schemas must be round-trippable, this tests that we are
// marshaling valid schemas.
func TestPackageBindsMarshalsAndRoundTrips(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		pkg := Package().Draw(t, "pkg")
		spec, err := pkg.MarshalSpec()
		require.NoError(t, err)
		pkg2, diags, err := schema.BindSpec(*spec, schema.Loader(nil), schema.ValidationOptions{})
		require.NoError(t, err)
		assert.Empty(t, diags)
		assert.Equal(t, pkg, pkg2)
	})
}

// TestPackageCoversAllShapes walks every generated package and verifies that
// across the rapid run we sampled at least one instance of each schema
// feature the generator is supposed to cover.
func TestPackageCoversAllShapes(t *testing.T) {
	t.Parallel()

	var seen shapes
	rapid.Check(t, func(t *rapid.T) {
		pkg := Package().Draw(t, "pkg")
		seen.observePackage(pkg)
	})

	assert.Empty(t, seen.missing())
}

// shapes tracks which schema features have been observed at least once
// across a rapid run.
type shapes struct {
	primitiveBoolean bool
	primitiveInteger bool
	primitiveNumber  bool
	primitiveString  bool
	array            bool
	mapType          bool
	objectRef        bool
	enumRef          bool
	archive          bool
	asset            bool
	jsonType         bool
	anyType          bool
	union            bool
	unionDisc        bool
	unionDiscMapping bool
	plain            bool
	secret           bool
	replaceOnChanges bool
	requiredProperty bool
	resource         bool
	stateInputs      bool
	complexEnum      bool
	complexObject    bool
}

func (s *shapes) missing() []string {
	checks := map[string]bool{
		"primitive:boolean":   s.primitiveBoolean,
		"primitive:integer":   s.primitiveInteger,
		"primitive:number":    s.primitiveNumber,
		"primitive:string":    s.primitiveString,
		"array":               s.array,
		"map":                 s.mapType,
		"objectRef":           s.objectRef,
		"enumRef":             s.enumRef,
		"archive":             s.archive,
		"asset":               s.asset,
		"json":                s.jsonType,
		"any":                 s.anyType,
		"union":               s.union,
		"union:discriminator": s.unionDisc,
		"union:mapping":       s.unionDiscMapping,
		"plain":               s.plain,
		"secret":              s.secret,
		"replaceOnChanges":    s.replaceOnChanges,
		"requiredProperty":    s.requiredProperty,
		"resource":            s.resource,
		"stateInputs":         s.stateInputs,
		"enumType":            s.complexEnum,
		"objectType":          s.complexObject,
	}
	var missing []string
	for name, ok := range checks {
		if !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func (s *shapes) observePackage(pkg *schema.Package) {
	for _, t := range pkg.Types {
		switch t := t.(type) {
		case *schema.ObjectType:
			s.complexObject = true
			for _, p := range t.Properties {
				s.observeProperty(p)
			}
		case *schema.EnumType:
			s.complexEnum = true
		}
	}
	for _, r := range pkg.Resources {
		s.resource = true
		if r.StateInputs != nil {
			s.stateInputs = true
		}
		for _, p := range r.Properties {
			s.observeProperty(p)
		}
		for _, p := range r.InputProperties {
			s.observeProperty(p)
		}
	}
	if pkg.Provider != nil {
		for _, p := range pkg.Provider.Properties {
			s.observeProperty(p)
		}
		for _, p := range pkg.Provider.InputProperties {
			s.observeProperty(p)
		}
	}
}

func (s *shapes) observeProperty(p *schema.Property) {
	if p.Secret {
		s.secret = true
	}
	if p.ReplaceOnChanges {
		s.replaceOnChanges = true
	}
	// A required property has its OptionalType wrapper stripped during bind,
	// so the absence of *OptionalType on the bound type indicates required.
	if _, isOpt := p.Type.(*schema.OptionalType); !isOpt {
		s.requiredProperty = true
	}
	if p.Plain {
		s.plain = true
	}
	s.observeType(p.Type)
}

func (s *shapes) observeType(typ schema.Type) {
	switch t := typ.(type) {
	case *schema.OptionalType:
		s.observeType(t.ElementType)
	case *schema.InputType:
		s.observeType(t.ElementType)
	case *schema.ArrayType:
		s.array = true
		s.observeType(t.ElementType)
	case *schema.MapType:
		s.mapType = true
		s.observeType(t.ElementType)
	case *schema.UnionType:
		s.union = true
		if t.Discriminator != "" {
			s.unionDisc = true
			if len(t.Mapping) > 0 {
				s.unionDiscMapping = true
			}
		}
		for _, e := range t.ElementTypes {
			s.observeType(e)
		}
	case *schema.ObjectType:
		s.objectRef = true
	case *schema.EnumType:
		s.enumRef = true
	case *schema.ResourceType:
		// not currently emitted by the generator
	case *schema.TokenType:
		s.observeType(t.UnderlyingType)
	}
	switch typ {
	case schema.BoolType:
		s.primitiveBoolean = true
	case schema.IntType:
		s.primitiveInteger = true
	case schema.NumberType:
		s.primitiveNumber = true
	case schema.StringType:
		s.primitiveString = true
	case schema.ArchiveType:
		s.archive = true
	case schema.AssetType:
		s.asset = true
	case schema.JSONType:
		s.jsonType = true
	case schema.AnyType:
		s.anyType = true
	}
}
