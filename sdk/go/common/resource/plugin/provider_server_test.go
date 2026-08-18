// Copyright 2023, Pulumi Corporation.
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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validate that Configure can read inputs from variables instead of args.
func TestProviderServer_Configure_variables(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ConfigureFunc: func(pm resource.PropertyMap) error {
			assert.Equal(t, map[string]any{
				"foo": "bar",
				"baz": 42.0,
				"qux": map[string]any{
					"a": "str",
					"b": true,
				},
			}, pm.Mappable())
			return nil
		},
	}
	srv := NewProviderServer(&provider)

	ctx := t.Context()
	_, err := srv.Configure(ctx, &pulumirpc.ConfigureRequest{
		Variables: map[string]string{
			"ns:foo": `"bar"`,
			"ns:baz": "42",
			"ns:qux": `{"a": "str", "b": true}`,
		},
	})
	require.NoError(t, err)
}

// stubProvider is a Provider implementation
// with support for stubbing out specific methods.
type stubProvider struct {
	Provider

	CreateFunc func(
		urn resource.URN,
		inputs resource.PropertyMap,
	) (CreateResponse, error)

	ReadFunc func(
		urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap,
	) (ReadResult, resource.Status, error)

	ConfigureFunc func(resource.PropertyMap) error

	ListFunc func(ListRequest) (*ListStream, error)
}

func (p *stubProvider) Configure(ctx context.Context, req ConfigureRequest) (ConfigureResponse, error) {
	if p.ConfigureFunc != nil {
		err := p.ConfigureFunc(req.Inputs)
		return ConfigureResponse{}, err
	}
	return p.Provider.Configure(ctx, req)
}

func (p *stubProvider) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if p.CreateFunc != nil {
		return p.CreateFunc(req.URN, req.Properties)
	}

	return p.Provider.Create(ctx, req)
}

func (p *stubProvider) Read(ctx context.Context, req ReadRequest) (ReadResponse, error) {
	if p.ReadFunc != nil {
		props, status, err := p.ReadFunc(req.URN, req.ID, req.Inputs, req.State)
		return ReadResponse{
			ReadResult: props,
			Status:     status,
		}, err
	}
	return p.Provider.Read(ctx, req)
}

func TestProviderServer_Create_mapsAlreadyExists(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	provider := stubProvider{
		CreateFunc: func(
			urn resource.URN,
			inputs resource.PropertyMap,
		) (CreateResponse, error) {
			return CreateResponse{}, &AlreadyExistsError{Cause: "conflicting remote resource"}
		},
	}

	srv := NewProviderServer(&provider)
	_, err := srv.Create(ctx, &pulumirpc.CreateRequest{
		Urn: "urn:pulumi:dev::project::pkg:index:Thing::thing",
	})
	require.Error(t, err)

	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, statusErr.Code())
	assert.Equal(t, "conflicting remote resource", statusErr.Message())
}

// When importing random passwords, the secret passed as "ID" should not leak in plain text into the final ID.
func TestProviderServer_Read_respects_ID(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	provider := stubProvider{
		ReadFunc: func(
			urn resource.URN, id resource.ID,
			inputs, state resource.PropertyMap,
		) (ReadResult, resource.Status, error) {
			return ReadResult{
				ID: resource.ID("none"),
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"result": resource.NewProperty(&resource.Secret{
						Element: resource.NewProperty(string(id)),
					}),
				}),
			}, resource.StatusOK, nil
		},
	}
	secret := "supersecretpassword"
	srv := NewProviderServer(&provider)
	resp, err := srv.Read(ctx, &pulumirpc.ReadRequest{
		Urn: "urn:pulumi:v2::re::random:index/randomPassword:RandomPassword::newPassword",
		Id:  secret,
	})
	require.NoError(t, err)
	require.NotEqual(t, secret, resp.Id)
}

func (p *stubProvider) List(ctx context.Context, req ListRequest) (*ListStream, error) {
	if p.ListFunc != nil {
		return p.ListFunc(req)
	}
	return p.Provider.List(ctx, req)
}

// fakeListServerStream captures messages sent by a server-streaming List call. Only Context and Send are exercised
// by the provider server; the embedded ServerStream is nil so other methods will panic if invoked.
type fakeListServerStream struct {
	grpc.ServerStream
	ctx  context.Context
	sent []*pulumirpc.ListResponse
}

func (s *fakeListServerStream) Context() context.Context { return s.ctx }

func (s *fakeListServerStream) Send(m *pulumirpc.ListResponse) error {
	s.sent = append(s.sent, m)
	return nil
}

func (s *fakeListServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeListServerStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeListServerStream) SetTrailer(metadata.MD)       {}

func TestProviderServer_List_streamsResults(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			assert.Equal(t, tokens.Type("pkgA:index:Thing"), req.Token)
			assert.Equal(t, int64(7), req.Limit)
			assert.Equal(t, int64(3), req.PageSize)
			assert.Equal(t, "cursor-in", req.ContinuationToken)
			return NewListStream([]ListResult{
				{ID: "id-a", Name: "alpha"},
				{ID: "id-b", Name: "beta"},
			}, "cursor-out"), nil
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{
		Token:             "pkgA:index:Thing",
		Limit:             7,
		PageSize:          3,
		ContinuationToken: "cursor-in",
	}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 3)
	r0 := stream.sent[0].GetResult()
	require.NotNil(t, r0)
	assert.Equal(t, "id-a", r0.GetId())
	assert.Equal(t, "alpha", r0.GetName())
	r1 := stream.sent[1].GetResult()
	require.NotNil(t, r1)
	assert.Equal(t, "id-b", r1.GetId())
	assert.Equal(t, "beta", r1.GetName())
	cont := stream.sent[2].GetContinuation()
	require.NotNil(t, cont)
	assert.Equal(t, "cursor-out", cont.GetContinuationToken())
}

// Results that the provider yields lazily via iter.Seq2 must reach the wire in order.
func TestProviderServer_List_streamsLazyResults(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			return &ListStream{
				Items: func(yield func(ListResult, error) bool) {
					for i := range 3 {
						if !yield(ListResult{ID: resource.ID(fmt.Sprintf("id-%d", i))}, nil) {
							return
						}
					}
				},
			}, nil
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{Token: "pkgA:index:Thing"}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 3)
	for i, msg := range stream.sent {
		r := msg.GetResult()
		require.NotNil(t, r)
		assert.Equal(t, fmt.Sprintf("id-%d", i), r.GetId())
	}
}

func TestProviderServer_List_sendsComputed(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			return NewComputedListStream(), nil
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{Token: "pkgA:index:Thing"}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 1)
	require.NotNil(t, stream.sent[0].GetComputed())
}

func TestProviderServer_List_sendsNothingForEmptyPage(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			return NewListStream(nil, ""), nil
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{Token: "pkgA:index:Thing"}, stream)
	require.NoError(t, err)
	assert.Empty(t, stream.sent)
}

func TestProviderServer_List_propagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("provider exploded")
	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			return nil, wantErr
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{Token: "pkgA:index:Thing"}, stream)
	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, stream.sent)
}

func TestProviderServer_List_propagatesItemError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("midstream error")
	provider := stubProvider{
		ListFunc: func(req ListRequest) (*ListStream, error) {
			return &ListStream{
				Items: func(yield func(ListResult, error) bool) {
					if !yield(ListResult{ID: "id-a"}, nil) {
						return
					}
					yield(ListResult{}, wantErr)
				},
			}, nil
		},
	}
	srv := NewProviderServer(&provider)

	stream := &fakeListServerStream{ctx: t.Context()}
	err := srv.List(&pulumirpc.ListRequest{Token: "pkgA:index:Thing"}, stream)
	assert.ErrorIs(t, err, wantErr)
	require.Len(t, stream.sent, 1)
	assert.Equal(t, "id-a", stream.sent[0].GetResult().GetId())
}
