// Copyright 2016-2022, Pulumi Corporation.
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

package rpcutil

import (
	"net"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
var maxRPCMessageSize = 1024 * 1024 * 400

// GrpcChannelOptions returns the defaultCallOptions with the max_receive_message_length increased to 400mb
// We want to increase the default message size as per pulumi/pulumi#2319
func GrpcChannelOptions() grpc.DialOption {
	return grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxRPCMessageSize))
}

// IsBenignCloseErr returns true if the error is "expected" upon shutdown of the server.
func IsBenignCloseErr(err error) bool {
	msg := err.Error()
	return strings.HasSuffix(msg, "use of closed network connection") ||
		strings.HasSuffix(msg, "grpc: the server has been stopped")
}

type ServeOptions struct {
	// Port to listen on. Passing 0 makes the system choose a port automatically.
	Port int

	// Initializer for the server. A typical Init registers handlers.
	Init func(*grpc.Server) error

	// If non-nil, Serve will gracefully terminate the server when Cancel is closed or receives true.
	Cancel chan bool

	// Options for serving gRPC.
	Options []grpc.ServerOption
}

type ServeHandle struct {
	// Port the server is listening on.
	Port int

	// The channel is non-nil and is closed when the server stops serving. The server will pass a non-nil error on
	// this channel if something went wrong in the background and it did not terminate gracefully.
	Done <-chan error
}

// ServeWithOptions creates a new gRPC server, calls opts.Init and listens on a TCP port.
func ServeWithOptions(opts ServeOptions) (ServeHandle, error) {
	h, _, err := serveWithOptions(opts)
	return h, err
}

func serveWithOptions(opts ServeOptions) (ServeHandle, chan error, error) {
	port := opts.Port

	// Listen on a TCP port, but let the kernel choose a free port for us.
	lis, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return ServeHandle{Port: port}, nil,
			errors.Errorf("failed to listen on TCP port ':%v': %v", port, err)
	}

	health := health.NewServer()

	// Now new up a gRPC server and register any RPC interfaces the caller wants.

	srv := grpc.NewServer(append(opts.Options, grpc.MaxRecvMsgSize(maxRPCMessageSize))...)

	if opts.Init != nil {
		if err := opts.Init(srv); err != nil {
			return ServeHandle{Port: port}, nil,
				errors.Errorf("failed to Init GRPC to register RPC handlers: %v", err)
		}
	}

	healthgrpc.RegisterHealthServer(srv, health) // enable health checks
	reflection.Register(srv)                     // enable reflection.

	// Set health checks for all the services that they are being served
	services := srv.GetServiceInfo()
	for serviceName := range services {
		health.SetServingStatus(serviceName, healthgrpc.HealthCheckResponse_SERVING)
	}

	// If the port was 0, look up what port the kernel chosen, by accessing the underlying TCP listener/address.
	if port == 0 {
		tcpl := lis.(*net.TCPListener)
		tcpa := tcpl.Addr().(*net.TCPAddr)
		port = tcpa.Port
	}

	cancel := opts.Cancel
	if cancel != nil {
		go func() {
			for v, ok := <-cancel; !v && ok; v, ok = <-cancel {
			}

			srv.GracefulStop()
		}()
	}

	// Finally, serve; this returns only once the server shuts down (e.g., due to a signal).
	done := make(chan error)
	go func() {
		if err := srv.Serve(lis); err != nil && !IsBenignCloseErr(err) {
			done <- errors.Errorf("stopped serving: %v", err)
		} else {
			done <- nil // send a signal so caller knows we're done, even though it's nil.
		}
		close(done)
	}()

	return ServeHandle{Port: port, Done: done}, done, nil
}

// Deprecated. Please use ServeWithOptions and OpenTracingServerInterceptorOptions.
func Serve(port int, cancel chan bool, registers []func(*grpc.Server) error,
	parentSpan opentracing.Span, options ...otgrpc.Option) (int, chan error, error) {

	opts := ServeOptions{
		Port:   port,
		Cancel: cancel,
		Init: func(s *grpc.Server) error {
			for _, r := range registers {
				if err := r(s); err != nil {
					return err
				}
			}
			return nil
		},
		Options: OpenTracingServerInterceptorOptions(parentSpan, options...),
	}

	handle, done, err := serveWithOptions(opts)
	if err != nil {
		return 0, nil, err
	}

	return handle.Port, done, nil
}
