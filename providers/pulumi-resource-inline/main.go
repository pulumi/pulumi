// Copyright 2025, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	providerName   = "inline"
	version        = "0.0.1"
	providerPrefix = "__provider_"
)

func main() {
	// TODO support `--version` flag

	err := provider.Main(providerName, func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return makeProvider(host, providerName, version)
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type Provider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host    *provider.HostClient
	name    string
	version string

	callbacksLock   sync.Mutex
	callbacks       map[string]*CallbacksClient // callbacks clients per target address
	grpcDialOptions DialOptions
}

// DialOptions returns dial options to be used for the gRPC client.
type DialOptions func(metadata any) []grpc.DialOption

func makeProvider(host *provider.HostClient, name, version string) (pulumirpc.ResourceProviderServer, error) {
	return &Provider{
		host:      host,
		name:      name,
		version:   version,
		callbacks: make(map[string]*CallbacksClient),
	}, nil
}

func (p *Provider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
	}, nil
}

func (p *Provider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	// TODO
	return &pulumirpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *Provider) Create(ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	callback, ok := getCallback(req.Properties, providerPrefix+"create")
	if !ok {
		return nil, errors.New("no create callback found for inline provider")
	}

	var response pulumirpc.CreateResponse
	if err := p.invokeCallback(ctx, callback, req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (p *Provider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	if callback, ok := getCallback(req.Inputs, providerPrefix+"read"); ok {
		var response pulumirpc.ReadResponse
		if err := p.invokeCallback(ctx, callback, req, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}

	return &pulumirpc.ReadResponse{
		Id:         req.GetId(),
		Properties: req.GetProperties(),
	}, nil
}

func (p *Provider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	if callback, ok := getCallback(req.News, providerPrefix+"diff"); ok {
		var response pulumirpc.DiffResponse
		if err := p.invokeCallback(ctx, callback, req, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}

	return &pulumirpc.DiffResponse{
		Changes: pulumirpc.DiffResponse_DIFF_NONE,
	}, nil
}

func (p *Provider) Update(ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	if callback, ok := getCallback(req.News, providerPrefix+"update"); ok {
		var response pulumirpc.UpdateResponse
		if err := p.invokeCallback(ctx, callback, req, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}

	return &pulumirpc.UpdateResponse{
		Properties: req.GetNews(),
	}, nil
}

func (p *Provider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	// FIXME: We need the _new_ inputs here, otherwise we won't be able to connect to the callback.
	// if callback, ok := getCallback(req.OldInputs, providerPrefix+"delete"); ok {
	// 	if err := p.invokeCallback(ctx, callback, req, &emptypb.Empty{}); err != nil {
	// 		return nil, err
	// 	}
	// }
	return &emptypb.Empty{}, nil
}

func (p *Provider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("unknown function '%s'", req.GetTok())
}

func (p *Provider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *Provider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	errs := []error{}
	for _, client := range p.callbacks {
		errs = append(errs, client.Close())
	}
	return &emptypb.Empty{}, errors.Join(errs...)
}

func (p *Provider) invokeCallback(ctx context.Context, callback callback, req proto.Message, res proto.Message) error {
	client, err := p.getCallbacksClient(callback.target)
	if err != nil {
		return err
	}

	request, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{
		Token:   callback.token,
		Request: request,
	})
	if err != nil {
		return err
	}

	return proto.Unmarshal(resp.Response, res)
}

// Get or allocate a new grpc client for the given callback address.
func (p *Provider) getCallbacksClient(target string) (*CallbacksClient, error) {
	p.callbacksLock.Lock()
	defer p.callbacksLock.Unlock()

	if client, has := p.callbacks[target]; has {
		return client, nil
	}

	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if p.grpcDialOptions != nil {
		opts := p.grpcDialOptions(map[string]any{
			"mode": "client",
			"kind": "callbacks",
		})
		dialOpts = append(dialOpts, opts...)
	}

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, err
	}

	client := NewCallbacksClient(conn)
	p.callbacks[target] = client
	return client, nil
}

type callback struct {
	target string
	token  string
}

func getCallback(props *structpb.Struct, name string) (callback, bool) {
	if props == nil {
		return callback{}, false
	}

	field, ok := props.Fields[name]
	if !ok {
		return callback{}, false
	}

	fieldStruct := field.GetStructValue()
	if fieldStruct == nil {
		return callback{}, false
	}

	sigValue, ok := fieldStruct.Fields[sig.Key]
	if !ok {
		return callback{}, false
	}
	sigValueStr := sigValue.GetStringValue()
	if sigValueStr != sig.Callback {
		return callback{}, false
	}

	targetValue, ok := fieldStruct.Fields["target"]
	if !ok {
		return callback{}, false
	}
	targetValueStr := targetValue.GetStringValue()
	if targetValueStr == "" {
		return callback{}, false
	}

	tokenValue, ok := fieldStruct.Fields["token"]
	if !ok {
		return callback{}, false
	}
	tokenValueStr := tokenValue.GetStringValue()
	if tokenValueStr == "" {
		return callback{}, false
	}

	return callback{
		target: targetValueStr,
		token:  tokenValueStr,
	}, true
}
