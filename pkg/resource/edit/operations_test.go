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

package edit

import (
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/stretchr/testify/assert"
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
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
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
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
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
	}, b64.NewBase64SecretsManager(), resources, nil)
}

func TestDeletion(t *testing.T) {
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

	err := DeleteResource(snap, b)
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, []*resource.State{pA, a, c}, snap.Resources)
}

func TestFailedDeletionProviderDependency(t *testing.T) {
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

	err := DeleteResource(snap, pA)
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

	err := DeleteResource(snap, a)
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
	pA := NewProviderResource("a", "p1", "0")
	a := NewResource("a", pA)
	a.Protect = true
	snap := NewSnapshot([]*resource.State{
		pA,
		a,
	})

	err := DeleteResource(snap, a)
	assert.Error(t, err)
	_, ok := err.(ResourceProtectedError)
	assert.True(t, ok)
}

func TestFailedDeletionParentDependency(t *testing.T) {
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

	err := DeleteResource(snap, a)
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

	err := UnprotectResource(snap, a)
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, []*resource.State{pA, a, b, c}, snap.Resources)
	assert.False(t, a.Protect)
}

func TestLocateResourceNotFound(t *testing.T) {
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

	// Baseline. Can locate resource A.
	resList := LocateResource(snap, a.URN)
	assert.Len(t, resList, 1)
	assert.Contains(t, resList, a)
	if t.Failed() {
		t.Fatal("Unable to find expected resource in initial snapshot.")
	}
	baselineResourceURN := resList[0].URN

	// The stack name and project are hard-coded in NewResource(...)
	assert.EqualValues(t, "test", baselineResourceURN.Stack())
	assert.EqualValues(t, "test", baselineResourceURN.Project())

	// Rename just the stack.
	t.Run("JustTheStack", func(t *testing.T) {
		err := RenameStack(snap, tokens.QName("new-stack"), tokens.PackageName(""))
		if err != nil {
			t.Fatalf("Error renaming stack: %v", err)
		}

		// Confirm the previous resource by URN isn't found.
		assert.Len(t, LocateResource(snap, baselineResourceURN), 0)

		// Confirm the resource has been renamed.
		updatedResourceURN := resource.NewURN(
			tokens.QName("new-stack"),
			"test", // project name stayed the same
			"" /*parent type*/, baselineResourceURN.Type(),
			baselineResourceURN.Name())
		assert.Len(t, LocateResource(snap, updatedResourceURN), 1)
	})

	// Rename the stack and project.
	t.Run("StackAndProject", func(t *testing.T) {
		err := RenameStack(snap, tokens.QName("new-stack2"), tokens.PackageName("new-project"))
		if err != nil {
			t.Fatalf("Error renaming stack: %v", err)
		}

		// Lookup the resource by URN, with both stack and project updated.
		updatedResourceURN := resource.NewURN(
			tokens.QName("new-stack2"),
			"new-project",
			"" /*parent type*/, baselineResourceURN.Type(),
			baselineResourceURN.Name())
		assert.Len(t, LocateResource(snap, updatedResourceURN), 1)
	})
}
