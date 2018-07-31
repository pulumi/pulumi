package deploy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/version"
)

func newResource(name string) *resource.State {
	ty := tokens.Type("test")
	return &resource.State{
		Type:    ty,
		URN:     resource.NewURN(tokens.QName("teststack"), tokens.PackageName("pkg"), ty, ty, tokens.QName(name)),
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
	}
}

func newSnapshot(resources []*resource.State) *Snapshot {
	return NewSnapshot(Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, resources)
}

// Tests that plan creation fails if there are any resources in an invalid state currently in the snapshot.
func TestInvalidResources(t *testing.T) {
	resourceA := newResource("a")
	resourceB := newResource("b")
	resourceB.Status = resource.OperationStatusCreating
	snap := newSnapshot([]*resource.State{
		resourceA,
		resourceB,
	})

	_, err := NewPlan(&plugin.Context{}, &Target{}, snap, &fixedSource{}, nil, false)
	if !assert.Error(t, err) {
		t.FailNow()
	}

	invalidErr, ok := err.(InvalidResourceError)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Len(t, invalidErr.InvalidResources, 1)
	assert.Equal(t, resourceB.URN, invalidErr.InvalidResources[0].URN)
	assert.Equal(t, resource.OperationStatusCreating, invalidErr.InvalidResources[0].Status)
}
