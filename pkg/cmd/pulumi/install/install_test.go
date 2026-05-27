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
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticNameResolver stands in for the plugin-running resolver, looking up
// the (name, resolvedSpec) result by the input's [workspace.PackageSpec.String].
func staticNameResolver(t *testing.T, mapping map[string]struct {
	name     string
	resolved workspace.PackageSpec
},
) func(workspace.PackageSpec) (string, workspace.PackageSpec, error) {
	t.Helper()
	return func(spec workspace.PackageSpec) (string, workspace.PackageSpec, error) {
		entry, ok := mapping[spec.String()]
		if !ok {
			return "", workspace.PackageSpec{}, fmt.Errorf("staticNameResolver: no mapping for %s", spec)
		}
		return entry.name, entry.resolved, nil
	}
}

// Two specs sharing one base plugin Source but with different Parameters must
// each land under their schema-discovered name so the Source-keyed local
// override in resolveStep can't alias one onto the other.
func TestAddRequiredSpecsToProject_MultipleParameterizationsOfSameSource(t *testing.T) {
	t.Parallel()

	archive := workspace.PackageSpec{
		Source: "terraform-provider", Parameters: []string{"hashicorp/archive"},
	}
	aws := workspace.PackageSpec{
		Source: "terraform-provider", Parameters: []string{"hashicorp/aws", "6.19.0"},
	}
	nameFor := staticNameResolver(t, map[string]struct {
		name     string
		resolved workspace.PackageSpec
	}{
		archive.String(): {name: "archive", resolved: archive},
		aws.String():     {name: "aws", resolved: aws},
	})

	proj := &workspace.Project{}
	added, err := addRequiredSpecsToProject(proj, []workspace.PackageSpec{archive, aws}, nameFor)
	require.NoError(t, err)
	require.True(t, added)

	assert.EqualExportedValues(t, map[string]workspace.PackageSpec{
		"archive": archive,
		"aws":     aws,
	}, proj.GetPackageSpecs())
}

func TestAddRequiredSpecsToProject_DedupesExactDuplicates(t *testing.T) {
	t.Parallel()

	existing := workspace.PackageSpec{
		Source:     "terraform-provider",
		Parameters: []string{"hashicorp/aws", "6.19.0"},
	}
	proj := &workspace.Project{}
	proj.AddPackage("aws", existing)

	resolver := func(workspace.PackageSpec) (string, workspace.PackageSpec, error) {
		t.Fatal("resolver must not be invoked for an already-present spec")
		return "", workspace.PackageSpec{}, errors.New("unreachable")
	}

	added, err := addRequiredSpecsToProject(proj, []workspace.PackageSpec{existing}, resolver)
	require.NoError(t, err)
	assert.False(t, added)
	require.Len(t, proj.GetPackageSpecs(), 1)
}

func TestAddRequiredSpecsToProject_NoSpecs(t *testing.T) {
	t.Parallel()

	resolver := func(workspace.PackageSpec) (string, workspace.PackageSpec, error) {
		t.Fatal("resolver must not be invoked when there are no specs")
		return "", workspace.PackageSpec{}, errors.New("unreachable")
	}

	proj := &workspace.Project{}
	added, err := addRequiredSpecsToProject(proj, nil, resolver)
	require.NoError(t, err)
	assert.False(t, added)
	assert.Empty(t, proj.GetPackageSpecs())
}

func TestAddRequiredSpecsToProject_PropagatesResolverError(t *testing.T) {
	t.Parallel()

	boom := errors.New("schema fetch failed")
	resolver := func(workspace.PackageSpec) (string, workspace.PackageSpec, error) {
		return "", workspace.PackageSpec{}, boom
	}

	proj := &workspace.Project{}
	_, err := addRequiredSpecsToProject(proj, []workspace.PackageSpec{
		{Source: "terraform-provider", Parameters: []string{"hashicorp/aws"}},
	}, resolver)
	require.ErrorIs(t, err, boom)
	assert.Empty(t, proj.GetPackageSpecs())
}
