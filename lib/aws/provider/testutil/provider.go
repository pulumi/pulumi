// Copyright 2016-2017, Pulumi Corporation
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

package testutil

import (
	"fmt"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
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
func ProviderTest(t *testing.T, resources map[string]Resource, steps []Step) map[string]*structpb.Struct {

	p := &providerTest{
		resources:            resources,
		namesInCreationOrder: []string{},
		ids:                  map[string]resource.ID{},
		props:                map[string]*structpb.Struct{},
		outProps:             map[string]*structpb.Struct{},
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
				id, props, outProps := createResource(t, res.Creator(p), provider, token)
				p.ids[res.Name] = resource.ID(id)
				p.namesInCreationOrder = append(p.namesInCreationOrder, res.Name)
				p.props[res.Name] = props
				p.outProps[res.Name] = outProps
				if id == "" {
					t.Fatal("expected to successfully create resource")
				}
			} else {
				oldProps := p.props[res.Name]
				ok, props, outProps := updateResource(t, string(id), oldProps, res.Creator(p), provider, token)
				if !ok {
					t.Fatal("expected to successfully update resource")
				}
				p.props[res.Name] = props
				p.outProps[res.Name] = outProps
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
			t.Fatal("expected to successfully delete resource")
		}
	}
	return p.outProps
}

// ProviderTestSimple takes a resource provider and array of resource steps and performs a Create, as many Updates
// as neeed, and finally a Delete operation on a single resouce of the given type to walk the resource through the
// resource lifecycle.  It also performs Check operations on each input state of the resource.
func ProviderTestSimple(t *testing.T, provider lumirpc.ResourceProviderServer, token tokens.Type, steps []interface{}) *structpb.Struct {
	resources := map[string]Resource{
		"testResource": {
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
	outProps := ProviderTest(t, resources, detailedSteps)
	return outProps["testResource"]
}

type providerTest struct {
	resources            map[string]Resource
	namesInCreationOrder []string
	ids                  map[string]resource.ID
	props                map[string]*structpb.Struct
	outProps             map[string]*structpb.Struct
}

func (p *providerTest) GetResourceID(name string) resource.ID {
	if id, ok := p.ids[name]; ok {
		return id
	}
	return resource.ID("")
}

var _ Context = &providerTest{}

func createResource(t *testing.T, res interface{}, provider lumirpc.ResourceProviderServer,
	token tokens.Type) (string, *structpb.Struct, *structpb.Struct) {
	props := plugin.MarshalProperties(nil, resource.NewPropertyMap(res), plugin.MarshalOptions{})
	fmt.Printf("[Provider Test]: Checking %v\n", token)
	checkResp, err := provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(token),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error checking table") {
		return "", nil, nil
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")
	fmt.Printf("[Provider Test]: Creating %v\n", token)
	resp, err := provider.Create(nil, &lumirpc.CreateRequest{
		Type:       string(token),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return "", nil, nil
	}
	if !assert.NotNil(t, resp, "expected a non-nil response") {
		return "", nil, nil
	}
	id := resp.Id
	fmt.Printf("[Provider Test]: Getting %v with id %v\n", token, id)
	getResp, err := provider.Get(nil, &lumirpc.GetRequest{
		Type: string(token),
		Id:   id,
	})
	if !assert.NoError(t, err, "expected no error reading resource") {
		return "", nil, nil
	}
	if !assert.NotNil(t, getResp, "expected a non-nil response reading the resources") {
		return "", nil, nil
	}
	return id, props, getResp.Properties
}

func updateResource(t *testing.T, id string, lastProps *structpb.Struct, res interface{},
	provider lumirpc.ResourceProviderServer, token tokens.Type) (bool, *structpb.Struct, *structpb.Struct) {
	newProps := plugin.MarshalProperties(nil, resource.NewPropertyMap(res), plugin.MarshalOptions{})
	fmt.Printf("[Provider Test]: Checking %v\n", token)
	checkResp, err := provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(token),
		Properties: newProps,
	})
	if !assert.NoError(t, err, "expected no error checking resource") {
		return false, nil, nil
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")
	fmt.Printf("[Provider Test]: Updating %v with id %v\n", token, id)
	_, err = provider.Update(nil, &lumirpc.UpdateRequest{
		Type: string(token),
		Id:   id,
		Olds: lastProps,
		News: newProps,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return false, nil, nil
	}
	fmt.Printf("[Provider Test]: Getting %v with id %v\n", token, id)
	getResp, err := provider.Get(nil, &lumirpc.GetRequest{
		Type: string(token),
		Id:   id,
	})
	if !assert.NoError(t, err, "expected no error reading resource") {
		return false, nil, nil
	}
	if !assert.NotNil(t, getResp, "expected a non-nil response reading the resources") {
		return false, nil, nil
	}
	return true, newProps, getResp.Properties
}

func deleteResource(t *testing.T, id string, provider lumirpc.ResourceProviderServer, token tokens.Type) bool {
	fmt.Printf("[Provider Test]: Deleting %v with id %v\n", token, id)
	_, err := provider.Delete(nil, &lumirpc.DeleteRequest{
		Type: string(token),
		Id:   id,
	})
	return assert.NoError(t, err, "expected no error deleting resource")
}

// CreateContext creates an AWS Context object for executing tests, and skips the test if the context cannot be
// created succefully, most likely because credentials are unavailable in the execution environment.
func CreateContext(t *testing.T) *awsctx.Context {
	if testing.Short() {
		t.Skip("skipping long running AWS provider test - run tests without -short to test providers")
	}
	ctx, err := awsctx.New(nil)
	if err != nil {
		t.Skipf("AWS context could not be acquired: %v", err)
	}
	return ctx
}
