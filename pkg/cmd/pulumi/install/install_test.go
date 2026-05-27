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

package install

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddRequiredSpecsToProject_MultipleParameterizationsOfSameSource pins down
// the fix for the language-host scenario where multiple PackageSpec entries
// share a single base plugin Source but parameterize it differently — e.g.
// `terraform-provider hashicorp/aws` and `terraform-provider hashicorp/archive`.
// The previous implementation deduped on Source alone, silently dropping all
// but the first, so `pulumi install` would generate only one SDK and `pulumi
// up` would later fail on the missing one.
func TestAddRequiredSpecsToProject_MultipleParameterizationsOfSameSource(t *testing.T) {
	t.Parallel()

	proj := &workspace.Project{}
	specs := []workspace.PackageSpec{
		{Source: "terraform-provider", Parameters: []string{"hashicorp/archive"}},
		{Source: "terraform-provider", Parameters: []string{"hashicorp/aws", "6.19.0"}},
	}

	require.True(t, addRequiredSpecsToProject(proj, specs))

	got := proj.GetPackageSpecs()
	require.Len(t, got, 2, "both parameterizations must be staged; got %v", got)

	bySpecString := map[string]workspace.PackageSpec{}
	for _, s := range got {
		bySpecString[s.String()] = s
	}
	for _, want := range specs {
		_, ok := bySpecString[want.String()]
		assert.Truef(t, ok, "missing staged spec %s in %v", want.String(), got)
	}
}

// TestAddRequiredSpecsToProject_DedupesExactDuplicates verifies that a spec
// already in the project's `packages` map (by full identity) is not re-added.
func TestAddRequiredSpecsToProject_DedupesExactDuplicates(t *testing.T) {
	t.Parallel()

	existing := workspace.PackageSpec{
		Source:     "terraform-provider",
		Parameters: []string{"hashicorp/aws", "6.19.0"},
	}
	proj := &workspace.Project{}
	proj.AddPackage("aws", existing)

	require.False(t, addRequiredSpecsToProject(proj, []workspace.PackageSpec{existing}),
		"identical spec must not be re-added")
	require.Len(t, proj.GetPackageSpecs(), 1)
}

// TestAddRequiredSpecsToProject_NoSpecs is a fast-path: empty input is a no-op.
func TestAddRequiredSpecsToProject_NoSpecs(t *testing.T) {
	t.Parallel()

	proj := &workspace.Project{}
	require.False(t, addRequiredSpecsToProject(proj, nil))
	require.Empty(t, proj.GetPackageSpecs())
}
