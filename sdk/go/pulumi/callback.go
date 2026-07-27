// Copyright 2016, Pulumi Corporation.
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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type callbackFunction = func(ctx context.Context, req []byte) (proto.Message, error)

type callbackServer struct {
	pulumirpc.UnsafeCallbacksServer

	stop   chan bool
	handle rpcutil.ServeHandle
	// advertiseHost is the host the engine is told to dial for callbacks. Empty means
	// loopback, which is correct whenever the engine shares this process's network
	// namespace — the same process, or a container the engine shares a netns with.
	advertiseHost string
	functions     map[string]callbackFunction
	functionsLock sync.RWMutex
}

// callbackAdvertiseHostEnvVar names the host the engine should dial to reach this
// process's callback server. It is set only when loopback would be wrong: the program
// runs in its own network namespace, so the engine dialing 127.0.0.1 reaches itself.
//
// Only a host is needed, not a full address. Unlike a plugin — which the engine must be
// able to address before it starts, and so needs a well-known port — a callback server
// reports its address to the engine after binding, so a kernel-chosen port is fine.
const callbackAdvertiseHostEnvVar = "PULUMI_CALLBACKS_ADVERTISE_HOST"

func newCallbackServer() (*callbackServer, error) {
	callbackServer := &callbackServer{
		functions: map[string]callbackFunction{},
		stop:      make(chan bool),
	}

	// Bind and advertise are derived from ONE read, deliberately: advertising a routable
	// host while still bound to loopback yields a connection refused indistinguishable
	// from advertising loopback in the first place.
	callbackServer.advertiseHost = os.Getenv(callbackAdvertiseHostEnvVar)
	listenAddress := ""
	if callbackServer.advertiseHost != "" {
		listenAddress = "0.0.0.0:0"
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel:        callbackServer.stop,
		ListenAddress: listenAddress,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterCallbacksServer(srv, callbackServer)
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, fmt.Errorf("could not start resource provider service: %w", err)
	}
	callbackServer.handle = handle

	return callbackServer, nil
}

// target is the address the engine dials to invoke a callback in this process.
func (s *callbackServer) target() string {
	host := s.advertiseHost
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(s.handle.Port))
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
		Target: s.target(),
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
