package testutil

import (
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"github.com/stretchr/testify/assert"
)

// ProviderTest takes a resource provider and array of resource steps and performs a Create, as many Udpates
// as neeed, and finally a Delete operation to walk the resource through the requested lifecycle.  It also
// performs Check operations on each provided resource.
func ProviderTest(t *testing.T, provider lumirpc.ResourceProviderServer, token tokens.Type, steps []interface{}) {

	p := &providerTest{
		t:         t,
		provider:  provider,
		token:     token,
		res:       nil,
		lastProps: nil,
	}

	id := ""
	for _, res := range steps {
		p.res = res
		if id == "" {
			id = p.createResource()
			if id == "" {
				t.Fatal("expected to succesfully create resource")
			}
		} else {
			if !p.updateResource(id) {
				t.Fatal("expected to succesfully update resource")
			}
		}
	}
	if !p.deleteResource(id) {
		t.Fatal("expected to succesfully delete resource")
	}
}

type providerTest struct {
	t         *testing.T
	provider  lumirpc.ResourceProviderServer
	token     tokens.Type
	res       interface{}
	lastProps *structpb.Struct
}

func (p *providerTest) createResource() string {
	props := resource.MarshalProperties(nil, resource.NewPropertyMap(p.res), resource.MarshalOptions{})
	checkResp, err := p.provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(p.token),
		Properties: props,
	})
	if !assert.NoError(p.t, err, "expected no error checking table") {
		return ""
	}
	assert.Equal(p.t, 0, len(checkResp.Failures), "expected no check failures")
	resp, err := p.provider.Create(nil, &lumirpc.CreateRequest{
		Type:       string(p.token),
		Properties: props,
	})
	if !assert.NoError(p.t, err, "expected no error creating resource") {
		return ""
	}
	if !assert.NotNil(p.t, resp, "expected a non-nil response") {
		return ""
	}
	id := resp.Id
	p.lastProps = props
	return id
}

func (p *providerTest) updateResource(id string) bool {
	newProps := resource.MarshalProperties(nil, resource.NewPropertyMap(p.res), resource.MarshalOptions{})
	checkResp, err := p.provider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(p.token),
		Properties: newProps,
	})
	if !assert.NoError(p.t, err, "expected no error checking resource") {
		return false
	}
	assert.Equal(p.t, 0, len(checkResp.Failures), "expected no check failures")
	_, err = p.provider.Update(nil, &lumirpc.UpdateRequest{
		Type: string(p.token),
		Id:   id,
		Olds: p.lastProps,
		News: newProps,
	})
	if !assert.NoError(p.t, err, "expected no error creating resource") {
		return false
	}
	p.lastProps = newProps
	return true
}

func (p *providerTest) deleteResource(id string) bool {
	_, err := p.provider.Delete(nil, &lumirpc.DeleteRequest{
		Type: string(p.token),
		Id:   id,
	})
	if !assert.NoError(p.t, err, "expected no error deleting resource") {
		return false
	}
	return true
}
