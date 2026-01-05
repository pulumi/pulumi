// Copyright 2016-2024, Pulumi Corporation.
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

package providers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Wraps a ResourceProviderServer to intercept and log incoming gRPC requests. The logged requests can later be
// retrieved and asserted against to test the details of gRPC wire representation.
//
// This is specifically useful for lower level tests, since not all actual providers use plugin.Provider consistently.
// Some are using older or modified versions, and gRPC level encoding details are pertinent.
//
// Currently only CheckConfig, Configure and DiffConfig requests are captured but this may be extended as needed.
type grpcCapturingProviderServer struct {
	pulumirpc.ResourceProviderServer
	sync.Mutex
	onRequest func(RPCRequest)
}

func (p *grpcCapturingProviderServer) logMessage(method RPCMethod, msg proto.Message) error {
	p.Lock()
	defer p.Unlock()
	r := newRPCRequest(method, msg)
	p.onRequest(r)
	return nil
}

func (p *grpcCapturingProviderServer) CheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	if err := p.logMessage(CheckConfigMethod, req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.CheckConfig(ctx, req)
}

func (p *grpcCapturingProviderServer) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (resp *pulumirpc.ConfigureResponse, err error) {
	if err := p.logMessage(ConfigureMethod, req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.Configure(ctx, req)
}

func (p *grpcCapturingProviderServer) DiffConfig(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	if err := p.logMessage(DiffConfigMethod, req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.DiffConfig(ctx, req)
}

type RPCRequest struct {
	Method  RPCMethod       `json:"method"`
	Message json.RawMessage `json:"message"`
}

func newRPCRequest(method RPCMethod, msg proto.Message) RPCRequest {
	req, err := protojson.Marshal(msg)
	contract.AssertNoErrorf(err, "protojson.Marshal should not fail")
	return RPCRequest{
		Method:  method,
		Message: req,
	}
}

// An enumeration for RPC methods.
type RPCMethod string

const (
	CheckConfigMethod RPCMethod = "CheckConfig"
	ConfigureMethod   RPCMethod = "Configure"
	DiffConfigMethod  RPCMethod = "DiffConfig"
)
