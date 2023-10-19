// Copyright 2016-2023, Pulumi Corporation.
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

package deploytest

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

func NewCallbacksServer() (*CallbackServer, error) {
	callbackServer := &CallbackServer{
		callbacks: make(map[string]func(args []resource.PropertyValue) ([]resource.PropertyValue, error)),
		stop:      make(chan bool),
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: callbackServer.stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterCallbacksServer(srv, callbackServer)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, fmt.Errorf("could not start resource provider service: %w", err)
	}
	callbackServer.handle = handle

	return callbackServer, nil
}

type CallbackServer struct {
	pulumirpc.UnsafeCallbacksServer

	stop      chan bool
	handle    rpcutil.ServeHandle
	callbacks map[string]func(args []resource.PropertyValue) ([]resource.PropertyValue, error)
}

func (s *CallbackServer) Close() error {
	s.stop <- true
	return <-s.handle.Done
}

func (s *CallbackServer) Allocate(
	callback func(args []resource.PropertyValue,
	) ([]resource.PropertyValue, error),
) (*pulumirpc.Callback, error) {
	token := uuid.NewString()
	s.callbacks[token] = callback
	return &pulumirpc.Callback{
		Target: fmt.Sprintf("127.0.0.1:%d", s.handle.Port),
		Token:  token,
	}, nil
}

func (s *CallbackServer) Invoke(
	ctx context.Context, req *pulumirpc.CallbackInvokeRequest,
) (*pulumirpc.CallbackInvokeResponse, error) {
	callback, ok := s.callbacks[req.Token]
	if !ok {
		return nil, nil
	}

	args := make([]resource.PropertyValue, len(req.Arguments))
	for i, arg := range req.Arguments {
		v, err := plugin.UnmarshalPropertyValue("Arguments", arg, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, err
		}
		args[i] = *v
	}

	result, err := callback(args)
	if err != nil {
		return nil, err
	}

	resp := &pulumirpc.CallbackInvokeResponse{
		Returns: make([]*structpb.Value, len(result)),
	}

	for i, ret := range result {
		v, err := plugin.MarshalPropertyValue("Returns", ret, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, err
		}
		resp.Returns[i] = v
	}

	return resp, nil
}
