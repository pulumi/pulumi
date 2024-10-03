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

package pulumi

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type callbackFunction = func(ctx context.Context, req []byte) (proto.Message, error)

type callbackServer struct {
	pulumirpc.UnsafeCallbacksServer

	stop          chan bool
	handle        rpcutil.ServeHandle
	functions     map[string]callbackFunction
	functionsLock sync.RWMutex
}

func newCallbackServer() (*callbackServer, error) {
	callbackServer := &callbackServer{
		functions: map[string]callbackFunction{},
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

func (s *callbackServer) RegisterCallback(function callbackFunction) (*pulumirpc.Callback, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	uuidString := uuid.String()
	s.functionsLock.Lock()
	defer s.functionsLock.Unlock()
	s.functions[uuidString] = function
	return &pulumirpc.Callback{
		Token:  uuidString,
		Target: "127.0.0.1:" + strconv.Itoa(s.handle.Port),
	}, nil
}

func (s *callbackServer) Invoke(
	ctx context.Context, req *pulumirpc.CallbackInvokeRequest,
) (*pulumirpc.CallbackInvokeResponse, error) {
	s.functionsLock.RLock()
	function, ok := s.functions[req.Token]
	s.functionsLock.RUnlock()
	if !ok {
		return nil, errors.New("callback function not found")
	}

	resp, err := function(ctx, req.Request)
	if err != nil {
		return nil, err
	}

	responseBytes, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %w", err)
	}

	return &pulumirpc.CallbackInvokeResponse{
		Response: responseBytes,
	}, nil
}
