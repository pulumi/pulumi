package deploy

import (
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
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

func newSnapshot(resources []*resource.State, ops []resource.Operation) *Snapshot {
	return NewSnapshot(Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, b64.NewBase64SecretsManager(), resources, ops)
}

func TestPendingOperationsDeployment(t *testing.T) {
	resourceA := newResource("a")
	resourceB := newResource("b")
	snap := newSnapshot([]*resource.State{
		resourceA,
	}, []resource.Operation{
		{
			Type:     resource.OperationTypeCreating,
			Resource: resourceB,
		},
	})

	_, err := NewDeployment(&plugin.Context{}, &Target{}, snap, &fixedSource{}, nil, false, nil)
	if !assert.Error(t, err) {
		t.FailNow()
	}

	invalidErr, ok := err.(PlanPendingOperationsError)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Len(t, invalidErr.Operations, 1)
	assert.Equal(t, resourceB.URN, invalidErr.Operations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeCreating, invalidErr.Operations[0].Type)
}
