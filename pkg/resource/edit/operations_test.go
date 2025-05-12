// Copyright 2016-2023, Pulumi Corporation.
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

package edit

import (
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NewResource(name string, provider *resource.State, deps ...resource.URN) *resource.State {
	prov := ""
	if provider != nil {
		p, err := providers.NewReference(provider.URN, provider.ID)
		if err != nil {
			panic(err)
		}
		prov = p.String()
	}

	t := tokens.Type("a:b:c")
	return &resource.State{
		Type:         t,
		URN:          resource.NewURN("test", "test", "", t, name),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
		Provider:     prov,
	}
}

func NewProviderResource(pkg, name, id string, deps ...resource.URN) *resource.State {
	t := providers.MakeProviderType(tokens.Package(pkg))
	return &resource.State{
		Type:         t,
		URN:          resource.NewURN("test", "test", "", t, name),
		ID:           resource.ID(id),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
	}
}

func NewSnapshot(resources []*resource.State) *deploy.Snapshot {
	return deploy.NewSnapshot(deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, b64.NewBase64SecretsManager(), resources, nil, deploy.SnapshotMetadata{})
}

func TestDeletion(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	err := DeleteResource(snap, b, nil, false)
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, []*resource.State{pA, a, c}, snap.Resources)
}

func TestDeletingDuplicateURNs(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)

	// Create duplicate resources.
	b1 := NewResource("b", pA)
	b2 := NewResource("b", pA)
	b3 := NewResource("b", pA)

	// ensure b1, b2, and b3 must have the same URN.
	bURN := b1.URN
	assert.Equal(t, bURN, b1.URN)
	assert.Equal(t, bURN, b2.URN)
	assert.Equal(t, bURN, b3.URN)

	// c exists to check behavior on b's dependents.
	c := NewResource("c", pA, bURN)

	// This test ensures that when targeting dependent resources, deleting a
	// resource with a redundant URN will not delete dependent resources in
	// state as it's ambiguous since another URN can satisfy the dependency.
	t.Run("do-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA, a, b1, b2, b3, c,
		})

		err := DeleteResource(snap, b1, nil, true /* targetDependents */)
		require.NoError(t, err)

		assert.Equal(t, []*resource.State{
			pA, a, b2, b3, c,
		}, snap.Resources)

		// Ensure that a pointer to b1 is not in the list.
		for _, s := range snap.Resources {
			assert.False(t, s == b1)
		}
	})

	// This test ensures that when targeting a resource with a redundant URN,
	// dependency checks should not block the resource from being deleted from state.
	t.Run("do-not-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA, a, b1, b2, b3, c,
		})

		err := DeleteResource(snap, b1, nil, false /* targetDependents */)
		require.NoError(t, err)

		assert.Equal(t, []*resource.State{
			pA, a, b2, b3, c,
		}, snap.Resources)

		// Ensure that a pointer to b1 is not in the list.
		for _, s := range snap.Resources {
			assert.False(t, s == b1)
		}
	})
}

func TestDeletingDuplicateProviderURN(t *testing.T) {
	t.Parallel()

	// Create duplicate provider resources
	pA0 := NewProviderResource("a", "p1", "0")
	pA1 := NewProviderResource("a", "p1", "1")

	// Create a resource that depends on the duplicate Provider.
	b0 := NewResource("b", pA0)
	b1 := NewResource("b", pA1)
	assert.Equal(t, b0.URN, b1.URN)

	c := NewResource("c", pA1, b0.URN)

	t.Run("do-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, b1, c,
		})

		err := DeleteResource(snap, pA0, nil, true /* targetDependents */)
		require.NoError(t, err)

		assert.Equal(t, []*resource.State{
			pA1, b1, c,
		}, snap.Resources)
	})

	t.Run("do-not-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, b1, c,
		})

		err := DeleteResource(snap, pA0, nil, false /* targetDependents */)
		require.ErrorContains(t, err,
			"Can't delete resource \"urn:pulumi:test::test::pulumi:providers:a::p1\" due to dependent resources")
	})

	t.Run("do-target-dependents-one-intermediate", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, c,
		})

		err := DeleteResource(snap, pA0, nil, true /* targetDependents */)
		require.NoError(t, err)
		assert.Equal(t, []*resource.State{
			pA1,
		}, snap.Resources)
	})

	t.Run("do-target-dependents-one-intermediate", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, c,
		})

		err := DeleteResource(snap, pA0, nil, false /* targetDependents */)
		require.ErrorContains(t, err,
			"Can't delete resource \"urn:pulumi:test::test::pulumi:providers:a::p1\" due to dependent resources")
	})
}

func TestDeletingDuplicateProviderURNWithDependents(t *testing.T) {
	t.Parallel()

	// Create duplicate provider resources
	pA0 := NewProviderResource("a", "p1", "0")
	pA1 := NewProviderResource("a", "p1", "1")

	// Create a resource that depends on the duplicate Provider.
	b0 := NewResource("b", pA0)

	c0 := NewProviderResource("c", "p1", "0", b0.URN)
	c1 := NewProviderResource("c", "p1", "1")

	d0 := NewResource("d", c0)
	d1 := NewResource("d", c1)

	t.Run("do-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, c0, c1, d0, d1,
		})

		err := DeleteResource(snap, pA0, nil, true /* targetDependents */)
		require.NoError(t, err)

		assert.Equal(t, []*resource.State{
			pA1, c1, d1,
		}, snap.Resources)
	})

	t.Run("do-not-target-dependents", func(t *testing.T) {
		t.Parallel()
		snap := NewSnapshot([]*resource.State{
			pA0, pA1, b0, c0, c1, d0, d1,
		})

		err := DeleteResource(snap, pA0, nil, false /* targetDependents */)
		require.ErrorContains(t, err,
			"Can't delete resource \"urn:pulumi:test::test::pulumi:providers:a::p1\" due to dependent resources")
	})
}

func TestDeletingDependencies(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	c := NewResource("c", pA, a.URN)
	d := NewResource("d", pA, c.URN)
	snap := NewSnapshot([]*resource.State{
		pA, a, b, c, d,
	})

	err := DeleteResource(snap, a, nil, true)
	require.NoError(t, err)

	assert.Equal(t, snap.Resources, []*resource.State{pA, b})
}

func TestFailedDeletionProviderDependency(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	err := DeleteResource(snap, pA, nil, false)
	assert.Error(t, err)
	depErr, ok := err.(ResourceHasDependenciesError)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Contains(t, depErr.Dependencies, a)
	assert.Contains(t, depErr.Dependencies, b)
	assert.Contains(t, depErr.Dependencies, c)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, []*resource.State{pA, a, b, c}, snap.Resources)
}

func TestFailedDeletionRegularDependency(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	err := DeleteResource(snap, a, nil, false)
	assert.Error(t, err)
	depErr, ok := err.(ResourceHasDependenciesError)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.NotContains(t, depErr.Dependencies, pA)
	assert.NotContains(t, depErr.Dependencies, a)
	assert.Contains(t, depErr.Dependencies, b)
	assert.NotContains(t, depErr.Dependencies, c)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, []*resource.State{pA, a, b, c}, snap.Resources)
}

func TestFailedDeletionProtected(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	a.Protect = true
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
	})

	err := DeleteResource(snap, a, nil, false)
	assert.Error(t, err)
	_, ok := err.(ResourceProtectedError)
	assert.True(t, ok)
}

func TestDeleteProtected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot)
	}{
		{
			"root-protected",
			func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot) {
				a.Protect = true
				protectedCount := 0
				err := DeleteResource(snap, a, func(s *resource.State) error {
					s.Protect = false
					protectedCount++
					return nil
				}, false)
				assert.NoError(t, err)
				assert.Equal(t, protectedCount, 1)
				assert.Equal(t, snap.Resources, []*resource.State{pA, b, c})
			},
		},
		{
			"root-and-branch",
			func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot) {
				a.Protect = true
				b.Protect = true
				c.Protect = true
				protectedCount := 0
				err := DeleteResource(snap, b, func(s *resource.State) error {
					s.Protect = false
					protectedCount++
					return nil
				}, true)
				assert.NoError(t, err)
				// 2 because we only plan to delete b and c. a is protected but not
				// scheduled for deletion, so we don't call the onProtect handler.
				assert.Equal(t, protectedCount, 2)
				assert.Equal(t, snap.Resources, []*resource.State{pA, a})
			},
		},
		{
			"branch",
			func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot) {
				b.Protect = true
				c.Protect = true
				protectedCount := 0
				err := DeleteResource(snap, c, func(s *resource.State) error {
					s.Protect = false
					protectedCount++
					return nil
				}, false)
				assert.NoError(t, err)
				assert.Equal(t, protectedCount, 1)
				assert.Equal(t, snap.Resources, []*resource.State{pA, a, b})
			},
		},
		{
			"no-permission-root",
			func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot) {
				c.Protect = true
				err := DeleteResource(snap, c, nil, false).(ResourceProtectedError)
				assert.Equal(t, ResourceProtectedError{
					Condemned: c,
				}, err)
			},
		},
		{
			"no-permission-branch",
			func(t *testing.T, pA, a, b, c *resource.State, snap *deploy.Snapshot) {
				c.Protect = true
				err := DeleteResource(snap, b, nil, true).(ResourceProtectedError)
				assert.Equal(t, ResourceProtectedError{
					Condemned: c,
				}, err)
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pA := NewProviderResource("a", "p1", "0")
			a := NewResource("a", pA)
			b := NewResource("b", pA)
			c := NewResource("c", pA, b.URN)
			snap := NewSnapshot([]*resource.State{
				pA,
				a,
				b,
				c,
			})

			tt.test(t, pA, a, b, c, snap)
		})
	}
}

func TestFailedDeletionParentDependency(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	b.Parent = a.URN
	c := NewResource("c", pA)
	c.Parent = a.URN
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	err := DeleteResource(snap, a, nil, false)
	assert.Error(t, err)
	depErr, ok := err.(ResourceHasDependenciesError)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.NotContains(t, depErr.Dependencies, pA)
	assert.NotContains(t, depErr.Dependencies, a)
	assert.Contains(t, depErr.Dependencies, b)
	assert.Contains(t, depErr.Dependencies, c)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, []*resource.State{pA, a, b, c}, snap.Resources)
}

func TestUnprotectResource(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	a.Protect = true
	b := NewResource("b", pA)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	UnprotectResource(a)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, []*resource.State{pA, a, b, c}, snap.Resources)
	assert.False(t, a.Protect)
}

func TestLocateResourceNotFound(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	ty := tokens.Type("a:b:c")
	urn := resource.NewURN("test", "test", "", ty, "not-present")
	resList := LocateResource(snap, urn)
	assert.Nil(t, resList)
}

func TestLocateResourceAmbiguous(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	aPending := NewResource("a", pA)
	aPending.Delete = true
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		aPending,
	})

	resList := LocateResource(snap, a.URN)
	assert.Len(t, resList, 2)
	assert.Contains(t, resList, a)
	assert.Contains(t, resList, aPending)
	assert.NotContains(t, resList, pA)
	assert.NotContains(t, resList, b)
}

func TestLocateResourceExact(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA)
	c := NewResource("c", pA)
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	resList := LocateResource(snap, a.URN)
	assert.Len(t, resList, 1)
	assert.Contains(t, resList, a)
}

func TestRenameStack(t *testing.T) {
	t.Parallel()

	locateResource := func(deployment *apitype.DeploymentV3, urn resource.URN) []apitype.ResourceV3 {
		if deployment == nil {
			return nil
		}

		var resources []apitype.ResourceV3
		for _, res := range deployment.Resources {
			if res.URN == urn {
				resources = append(resources, res)
			}
		}

		return resources
	}

	newResource := func(name string, provider *apitype.ResourceV3, deps ...resource.URN) apitype.ResourceV3 {
		prov := ""
		if provider != nil {
			p, err := providers.NewReference(provider.URN, provider.ID)
			if err != nil {
				panic(err)
			}
			prov = p.String()
		}

		t := tokens.Type("a:b:c")
		return apitype.ResourceV3{
			Type:         t,
			URN:          resource.NewURN("test", "test", "", t, name),
			Inputs:       map[string]interface{}{},
			Outputs:      map[string]interface{}{},
			Dependencies: deps,
			Provider:     prov,
		}
	}

	newProviderResource := func(pkg, name, id string, deps ...resource.URN) apitype.ResourceV3 {
		t := providers.MakeProviderType(tokens.Package(pkg))
		return apitype.ResourceV3{
			Type:         t,
			URN:          resource.NewURN("test", "test", "", t, name),
			ID:           resource.ID(id),
			Inputs:       map[string]interface{}{},
			Outputs:      map[string]interface{}{},
			Dependencies: deps,
		}
	}

	newDeployment := func(resources []apitype.ResourceV3) *apitype.DeploymentV3 {
		return &apitype.DeploymentV3{
			Manifest: apitype.ManifestV1{
				Time:    time.Now(),
				Version: version.Version,
				Plugins: nil,
			},
			Resources: resources,
		}
	}

	pA := newProviderResource("a", "p1", "0")
	a := newResource("a", &pA)
	b := newResource("b", &pA)
	c := newResource("c", &pA)
	deployment := newDeployment([]apitype.ResourceV3{
		pA,
		a,
		b,
		c,
	})

	// Baseline. Can locate resource A.
	resList := locateResource(deployment, a.URN)
	assert.Len(t, resList, 1)
	assert.Contains(t, resList, a)
	if t.Failed() {
		t.Fatal("Unable to find expected resource in initial checkpoint.")
	}
	baselineResourceURN := resList[0].URN

	// The stack name and project are hard-coded in NewResource(...)
	assert.EqualValues(t, "test", baselineResourceURN.Stack())
	assert.EqualValues(t, "test", baselineResourceURN.Project())

	// Rename just the stack.
	//nolint:paralleltest // uses shared stack
	t.Run("JustTheStack", func(t *testing.T) {
		err := RenameStack(deployment, tokens.MustParseStackName("new-stack"), tokens.PackageName(""))
		if err != nil {
			t.Fatalf("Error renaming stack: %v", err)
		}

		// Confirm the previous resource by URN isn't found.
		assert.Len(t, locateResource(deployment, baselineResourceURN), 0)

		// Confirm the resource has been renamed.
		updatedResourceURN := resource.NewURN(
			tokens.QName("new-stack"),
			"test", // project name stayed the same
			"" /*parent type*/, baselineResourceURN.Type(),
			baselineResourceURN.Name())
		assert.Len(t, locateResource(deployment, updatedResourceURN), 1)
	})

	// Rename the stack and project.
	//nolint:paralleltest // uses shared stack
	t.Run("StackAndProject", func(t *testing.T) {
		err := RenameStack(deployment, tokens.MustParseStackName("new-stack2"), tokens.PackageName("new-project"))
		if err != nil {
			t.Fatalf("Error renaming stack: %v", err)
		}

		// Lookup the resource by URN, with both stack and project updated.
		updatedResourceURN := resource.NewURN(
			tokens.QName("new-stack2"),
			"new-project",
			"" /*parent type*/, baselineResourceURN.Type(),
			baselineResourceURN.Name())
		assert.Len(t, locateResource(deployment, updatedResourceURN), 1)
	})
}
