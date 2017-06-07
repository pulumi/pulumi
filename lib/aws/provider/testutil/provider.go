package testutil

import (
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"github.com/stretchr/testify/assert"
)

type Context interface {
	GetResourceID(name string) resource.ID
}

type Resource struct {
	Provider lumirpc.ResourceProviderServer
	Token    tokens.Type
}

type ResourceGenerator struct {
	Name    string
	Creator func(ctx Context) interface{}
}

type Step []ResourceGenerator

// ProviderTest walks through Create, Update and Delete operations for a collection of resources.  The provided
// resources map must contain the provider and tokens for each named resource to be created during the test.
// Each step of the test can provide values for any subset of the named resources, causeing those resources to
// be created or updated as needed.  After walking through each step, all of the created resources are deleted.
// Check operations are performed on all provided resource inputs during the test.
// performs Check operations on each provided resource.
func ProviderTest(t *testing.T, resources map[string]Resource, steps []Step) {

	p := &providerTest{
		resources:            resources,
		namesInCreationOrder: []string{},
		ids:                  map[string]resource.ID{},
		props:                map[string]*structpb.Struct{},
	}

	// For each step, create or update all listed resources
	for _, step := range steps {
		for _, res := range step {
			currentResource, ok := resources[res.Name]
			if !ok {
				t.Fatalf("expected resource to have been pre-declared: %v", res.Name)
			}
			provider := currentResource.Provider
			token := currentResource.Token
			if id, ok := p.ids[res.Name]; !ok {
				id, props := createResource(t, res.Creator(p), provider, token)
				p.ids[res.Name] = resource.ID(id)
				p.namesInCreationOrder = append(p.namesInCreationOrder, res.Name)
				p.props[res.Name] = props
				if id == "" {
					t.Fatal("expected to succesfully create resource")
				}
			} else {
				oldProps := p.props[res.Name]
				ok, props := updateResource(t, string(id), oldProps, res.Creator(p), provider, token)
				if !ok {
					t.Fatal("expected to succesfully update resource")
				}
				p.props[res.Name] = props
			}
		}
	}
	// Delete resources in the opposite order they were created
	for i := len(p.namesInCreationOrder) - 1; i >= 0; i-- {
		name := p.namesInCreationOrder[i]
		id := p.ids[name]
		provider := resources[name].Provider
		token := resources[name].Token
		ok := deleteResource(t, string(id), provider, token)
		if !ok {
			t.Fatal("expected to succesfully delete resource")
		}
	}
}

// ProviderTestSimple takes a resource provider and array of resource steps and performs a Create, as many Udpates
// as neeed, and finally a Delete operation on a single resouce of the given type to walk the resource through the
// resource lifecycle.  It also performs Check operations on each input state of the resource.
func ProviderTestSimple(t *testing.T, provider lumirpc.ResourceProviderServer, token tokens.Type, steps []interface{}) {
	resources := map[string]Resource{
		"testResource": Resource{
			Provider: provider,
			Token:    token,
		},
	}
	detailedSteps := []Step{}
	for _, step := range steps {
		curStep := step
		detailedSteps = append(detailedSteps, []ResourceGenerator{
			{
				Name: "testResource",
				Creator: func(ctx Context) interface{} {
					return curStep
				},
			},
		})
	}
	ProviderTest(t, resources, detailedSteps)
}

type providerTest struct {
	resources            map[string]Resource
	namesInCreationOrder []string
	ids                  map[string]resource.ID
	props                map[string]*structpb.Struct
}

func (p *providerTest) GetResourceID(name string) resource.ID {
	if id, ok := p.ids[name]; ok {
		return id
	}
	return resource.ID("")
}

var _ Context = &providerTest{}

func createResource(t *testing.T, res interface{}, provider lumirpc.ResourceProviderServer, token tokens.Type) (string, *structpb.Struct) {
	props := resource.MarshalProperties(nil, resource.NewPropertyMap(res), resource.MarshalOptions{})
	checkResp, err := provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(token),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error checking table") {
		return "", nil
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")
	resp, err := provider.Create(nil, &lumirpc.CreateRequest{
		Type:       string(token),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return "", nil
	}
	if !assert.NotNil(t, resp, "expected a non-nil response") {
		return "", nil
	}
	id := resp.Id
	return id, props
}

func updateResource(t *testing.T, id string, lastProps *structpb.Struct, res interface{}, provider lumirpc.ResourceProviderServer, token tokens.Type) (bool, *structpb.Struct) {
	newProps := resource.MarshalProperties(nil, resource.NewPropertyMap(res), resource.MarshalOptions{})
	checkResp, err := provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(token),
		Properties: newProps,
	})
	if !assert.NoError(t, err, "expected no error checking resource") {
		return false, nil
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")
	_, err = provider.Update(nil, &lumirpc.UpdateRequest{
		Type: string(token),
		Id:   id,
		Olds: lastProps,
		News: newProps,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return false, nil
	}
	return true, newProps
}

func deleteResource(t *testing.T, id string, provider lumirpc.ResourceProviderServer, token tokens.Type) bool {
	_, err := provider.Delete(nil, &lumirpc.DeleteRequest{
		Type: string(token),
		Id:   id,
	})
	if !assert.NoError(t, err, "expected no error deleting resource") {
		return false
	}
	return true
}
