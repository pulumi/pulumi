package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func createSnapshot() Snapshot {
	resourceUrns := []resource.URN{
		resource.NewURN("stack", "test", "typ", "aws:resource", "bar"),
		resource.NewURN("stack", "test", "typ", "aws:resource", "aname"),
		resource.NewURN("stack", "test", "typ", "azure:resource", "bar"),
	}
	resources := []*resource.State{}
	for _, u := range resourceUrns {
		resources = append(resources, &resource.State{URN: u})
	}
	return Snapshot{Resources: resources}
}

func createSnapshotPtr() *Snapshot {
	s := createSnapshot()
	return &s
}

func TestSnapshotNormalizeURNReferences(t *testing.T) {
	t.Parallel()
	s1 := createSnapshotPtr()
	s1n, err := s1.NormalizeURNReferences()
	assert.NoError(t, err)
	assert.Same(t, s1, s1n)

	s2 := createSnapshotPtr()
	r0 := s2.Resources[0]
	r0.Aliases = []resource.URN{r0.URN}
	s2.Resources[2].Parent = r0.URN
	r0.URN += "!"
	s2n, err := s2.NormalizeURNReferences()
	assert.NoError(t, err)
	assert.NotSame(t, s2, s2n)
	// before normalize in s2, Parent link uses outdated URL
	assert.Equal(t, s2.Resources[2].Parent+"!", s2.Resources[0].URN)
	// after normalize in s2n, Parent link uses the real URL rewritten via aliases
	assert.Equal(t, s2n.Resources[2].Parent, s2n.Resources[0].URN)
}

func TestSnapshotWithUpdatedResources(t *testing.T) {
	t.Parallel()
	s1 := createSnapshotPtr()

	s := s1.withUpdatedResources(func(r *resource.State) *resource.State {
		return r
	})
	assert.Same(t, s, s1)

	s = s1.withUpdatedResources(func(r *resource.State) *resource.State {
		out := r.Copy()
		out.URN += "!"
		return out
	})
	assert.NotSame(t, s, s1)
	assert.Equal(t, s1.Resources[0].URN+"!", s.Resources[0].URN)
}
