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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func NewCallbacksServer() (*CallbackServer, error) {
	callbackServer := &CallbackServer{
		callbacks: make(map[string]func(args []byte) (proto.Message, error)),
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
	callbacks map[string]func(req []byte) (proto.Message, error)
}

func (s *CallbackServer) Close() error {
	s.stop <- true
	return <-s.handle.Done
}

func (s *CallbackServer) Allocate(
	callback func(args []byte) (proto.Message, error),
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

	response, err := callback(req.Request)
	if err != nil {
		return nil, err
	}

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %w", err)
	}

	return &pulumirpc.CallbackInvokeResponse{
		Response: responseBytes,
	}, nil
}
