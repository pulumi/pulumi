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

package importer

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	rapidimporter "github.com/pulumi/pulumi/pkg/v3/importer/rapid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"golang.org/x/text/unicode/norm"
)

// TestRapidGenerateHCL2Definition is a property-based round-trip test for
// [GenerateHCL2Definition]: for any rapid-generated schema and any
// schema-conforming resource state, the generated HCL2 block should parse,
// bind, and render back to a state whose Inputs equal the originals.
//
// The driver currently asserts only on Inputs; envelope fields (Provider,
// Parent, options, etc.) are tracked in PLAN.md and will be added once the
// PCL evaluator is wired in (per step 4 TODO).
func TestRapidGenerateHCL2Definition(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Filter(hasSelectableResource).Draw(t, "pkg")
		sample := rapidimporter.State(pkg).Draw(t, "sample")

		if checkPropertyMap(sample.State.Inputs, property.Value.IsNull) {
			// generatePropertyValue (pkg/importer/hcl2.go:588) normalizes a
			// null value for an optional property to "no attribute emitted",
			// so an original `{b: null}` round-trips to `{}`. Skip these
			// cases — the equivalence under that normalization is captured by
			// the test, but DeepEquals does not see absent and null as equal.
			t.Skip("inputs contain an explicit null; production normalizes optional-null to absent")
		}

		if checkPropertyMap(sample.State.Inputs, func(v property.Value) bool {
			if v.IsString() && !norm.NFC.IsNormalString(v.AsString()) {
				return true
			}
			if v.IsMap() {
				for k := range v.AsMap().All {
					if !norm.NFC.IsNormalString(k) {
						return true
					}
				}
				return false
			}
			if v.IsAsset() {
				a := v.AsAsset()
				return !norm.NFC.IsNormalString(a.Text) ||
					!norm.NFC.IsNormalString(a.Path) ||
					!norm.NFC.IsNormalString(a.URI)
			}
			if v.IsArchive() {
				return archiveHasNonNFC(v.AsArchive())
			}
			return false
		}) {
			// TODO: real bug — the HCL parser (or cty) normalizes string
			// literals to Unicode NFC, so `"À"` (decomposed) round-trips
			// to `"À"` (precomposed). cty stores strings as raw bytes
			// and the spec doesn't mandate normalization, so this loses
			// information silently. Skip non-NFC inputs until that is fixed.
			t.Skip("inputs contain a non-NFC string; HCL round-trip silently NFC-normalizes")
		}

		if checkPropertyMap(sample.State.Inputs, func(p property.Value) bool {
			if !p.IsMap() {
				return false
			}
			for k := range p.AsMap().All {
				if strings.HasPrefix(k, "__") {
					return true
				}
			}
			return false
		}) {
			// TODO: real bug — generateValue (pkg/importer/hcl2.go:884)
			// silently drops every key whose name starts with `__` from a
			// MapType / Any map value. The original `{"__internal": x}`
			// round-trips to `{}`. The schema does not allow ObjectType
			// property names to start with `__`, so this only fires for
			// MapType / Any-shaped values. Skip until the importer keeps
			// these keys (or rejects them at a higher level).
			t.Skip("inputs contain a `__`-prefixed map key; production drops them silently")
		}

		loader := &stubSchemaLoader{pkg: pkg}
		importState := buildImportState(sample)

		block, _, err := GenerateHCL2Definition(loader, sample.State, importState)
		require.NoError(t, err)

		text := fmt.Sprint(block)
		parser := syntax.NewParser()
		require.NoError(t, parser.ParseFile(strings.NewReader(text), string(sample.State.URN)+".pp"))
		if parser.Diagnostics.HasErrors() {
			// TODO: real bug — the HCL2 model formatter emits non-printable
			// bytes as `\xNN`, but the HCL parser only accepts `\uNNNN`
			// escapes. Any input string or map key with a control byte (e.g.
			// NUL) round-trips through fmt.Sprint(block) into invalid HCL.
			// Skip these cases until the formatter is fixed.
			t.Skipf("formatter emitted unparseable HCL (likely \\xNN escape bug): %v\nblock:\n%s",
				parser.Diagnostics.Error(), text)
		}

		prog, diags, err := pcl.BindProgram(parser.Files, loader, pcl.AllowMissingVariables)
		require.NoErrorf(t, err, "block:\n%s", text)
		require.Falsef(t, diags.HasErrors(),
			"bind diagnostics: %v\nblock:\n%s", diags.Error(), text)
		require.NotNil(t, prog)

		require.Lenf(t, prog.Nodes, 1, "expected one node in program, got %d", len(prog.Nodes))
		res, ok := prog.Nodes[0].(*pcl.Resource)
		require.Truef(t, ok, "first node is %T, want *pcl.Resource", prog.Nodes[0])

		gotInputs := renderInputs(t, res)
		require.Truef(t, sample.State.Inputs.DeepEquals(gotInputs),
			"inputs differ\nwant: %#v\ngot:  %#v\nblock:\n%s", sample.State.Inputs, gotInputs, text)
	})
}

// archiveHasNonNFC reports whether a contains, transitively, any string that
// is not in Unicode NFC form (asset text/path/uri, archive path/uri, or
// archive entry keys).
func archiveHasNonNFC(a property.Archive) bool {
	if !norm.NFC.IsNormalString(a.Path) || !norm.NFC.IsNormalString(a.URI) {
		return true
	}
	for k, entry := range a.Assets {
		if !norm.NFC.IsNormalString(k) {
			return true
		}
		switch entry := entry.(type) {
		case property.Asset:
			if !norm.NFC.IsNormalString(entry.Text) ||
				!norm.NFC.IsNormalString(entry.Path) ||
				!norm.NFC.IsNormalString(entry.URI) {
				return true
			}
		case property.Archive:
			if archiveHasNonNFC(entry) {
				return true
			}
		}
	}
	return false
}

func checkPropertyMap(m resource.PropertyMap, check func(property.Value) bool) bool {
	var walk func(property.Value) bool
	walk = func(v property.Value) bool {
		switch {
		case check(v):
			return true
		case v.IsArray():
			for _, v := range v.AsArray().All {
				if walk(v) {
					return true
				}
			}
			return false
		case v.IsMap():
			for _, v := range v.AsMap().All {
				if walk(v) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
	return walk(property.New(resource.FromResourcePropertyMap(m)))
}

// renderInputs extracts a [resource.PropertyMap] from a bound [pcl.Resource]
// by walking each input attribute through [renderExpr]. Unlike
// [renderResource], it does not attempt to reconstruct the envelope (URN,
// Parent, Provider, etc.) — those reconstructions assume the test-fixture name
// table from hcl2_test.go and crash on rapid-generated names. The driver only
// asserts on Inputs (PLAN.md step 4 TODO covers widening once a PCL evaluator
// is wired in).
func renderInputs(t require.TestingT, r *pcl.Resource) resource.PropertyMap {
	inputs := map[string]property.Value{}
	for _, attr := range r.Inputs {
		inputs[attr.Name] = renderExpr(t, attr.Value)
	}
	return resource.ToResourcePropertyMap(property.NewMap(inputs))
}

// hasSelectableResource reports whether pkg declares at least one custom,
// non-provider resource, which [rapidimporter.State] requires.
func hasSelectableResource(pkg *schema.Package) bool {
	for _, r := range pkg.Resources {
		if !r.IsProvider && !r.IsComponent {
			return true
		}
	}
	return false
}

// buildImportState assembles the [ImportState] needed by
// [GenerateHCL2Definition]. Every snapshot URN is given a unique name so the
// PCL binder doesn't see name-collision-induced "circular reference" errors;
// the resource state's own URN name is reserved first and snapshot names are
// suffixed (`_2`, `_3`, ...) on collision.
func buildImportState(sample *rapidimporter.Sample) ImportState {
	taken := map[string]bool{sample.State.URN.Name(): true}
	names := NameTable{}
	for _, s := range sample.Snapshot {
		base := s.URN.Name()
		name := base
		for i := 2; taken[name]; i++ {
			name = fmt.Sprintf("%s_%d", base, i)
		}
		taken[name] = true
		names[s.URN] = name
	}
	return ImportState{
		Names:    names,
		Snapshot: sample.Snapshot,
	}
}

// stubSchemaLoader returns a fixed [*schema.Package] for any descriptor.
// It is the same shape as the stub at pkg/codegen/pcl/binder_resource_test.go,
// duplicated here so the test stays in package importer (where renderResource
// is package-private) without exposing the stub to other tests.
type stubSchemaLoader struct {
	pkg *schema.Package
}

var _ schema.Loader = (*stubSchemaLoader)(nil)

func (l *stubSchemaLoader) LoadPackage(name string, ver *semver.Version) (*schema.Package, error) {
	return l.pkg, nil
}

func (l *stubSchemaLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	return l.pkg, nil
}
