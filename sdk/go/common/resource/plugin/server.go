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

package plugin

import (
	"fmt"
	"io"

	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
)

// GrpcServer is a standard Pulumi style gRPC server that can be used to serve gRPC services.
type GrpcServer struct {
	io.Closer

	cancel chan bool
	handle rpcutil.ServeHandle
}

// NewServer creates a new GrpcServer wired up to the given services and context.
func NewServer(ctx *Context, registrations ...func(server *grpc.Server)) (*GrpcServer, error) {
	cancel := make(chan bool)

	// Fire up a gRPC server and start listening for incomings.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			for _, registration := range registrations {
				registration(srv)
			}
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(ctx.tracingSpan),
	})
	if err != nil {
		return nil, err
	}

	return &GrpcServer{
		cancel: cancel,
		handle: handle,
	}, nil
}

func (s *GrpcServer) Close() error {
	if s.cancel != nil {
		s.cancel <- true
		err := <-s.handle.Done
		s.cancel = nil
		return err
	}
	return nil
}

func (s *GrpcServer) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", s.handle.Port)
}
