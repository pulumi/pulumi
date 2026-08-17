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

package rapidimporter

import (
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidresource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// TestRapidState draws a schema, then a Sample from the State generator, and
// asserts that:
//   - the state's Type matches a non-provider resource declared in the package;
//   - the state's Inputs are structurally typed against that resource's input
//     properties;
//   - the snapshot contains the provider referenced by State.Provider.
func TestRapidState(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Filter(hasSelectableResource).Draw(t, "pkg")
		sample := State(pkg).Draw(t, "sample")

		r := findResourceByToken(pkg, sample.State.Type)
		require.NotNilf(t, r, "state type %q not declared in package", sample.State.Type)
		require.Falsef(t, r.IsProvider, "state type %q is a provider", sample.State.Type)

		inputs := resource.FromResourcePropertyMap(sample.State.Inputs)
		require.Truef(t,
			rapidresource.MapStructurallyTyped(inputs, r.InputProperties),
			"inputs %v do not match input properties of %q", inputs, r.Token)

		require.Truef(t,
			snapshotContainsProvider(sample.Snapshot, sample.State.Provider),
			"snapshot does not contain provider %q", sample.State.Provider)
	})
}

func hasSelectableResource(pkg *schema.Package) bool {
	return len(selectableResources(pkg)) > 0
}

func findResourceByToken(pkg *schema.Package, typ tokens.Type) *schema.Resource {
	for _, r := range pkg.Resources {
		if r.Token == string(typ) {
			return r
		}
	}
	return nil
}

func snapshotContainsProvider(snapshot []*pkgresource.State, providerRef string) bool {
	ref, err := providers.ParseReference(providerRef)
	if err != nil {
		return false
	}
	for _, s := range snapshot {
		if s.URN == ref.URN() && s.ID == ref.ID() {
			return true
		}
	}
	return false
}
