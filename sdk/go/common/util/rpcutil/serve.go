// Copyright 2016-2018, Pulumi Corporation.
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

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
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

// Serve creates a new gRPC server, calls out to the supplied registration functions to bind interfaces, and then
// listens on the supplied TCP port.  If the caller wishes for the kernel to choose a free port automatically, pass 0 as
// the port number.  The return values are: the chosen port (the same as supplied if non-0), a channel that may
// eventually return an error, and an error, in case something went wrong.  The channel is non-nil and waits until
// the server is finished, in the case of a successful launch of the RPC server.
func Serve(port int, cancel chan bool, registers []func(*grpc.Server) error,
	parentSpan opentracing.Span) (int, chan error, error) {

	// Listen on a TCP port, but let the kernel choose a free port for us.
	lis, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return port, nil, errors.Errorf("failed to listen on TCP port ':%v': %v", port, err)
	}

	// Now new up a gRPC server and register any RPC interfaces the caller wants.
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(OpenTracingServerInterceptor(parentSpan)),
		grpc.MaxRecvMsgSize(maxRPCMessageSize),
	)
	for _, register := range registers {
		if err := register(srv); err != nil {
			return port, nil, errors.Errorf("failed to register RPC handler: %v", err)
		}
	}
	reflection.Register(srv) // enable reflection.

	// If the port was 0, look up what port the kernel chosen, by accessing the underlying TCP listener/address.
	if port == 0 {
		tcpl := lis.(*net.TCPListener)
		tcpa := tcpl.Addr().(*net.TCPAddr)
		port = tcpa.Port
	}

	// If the caller provided a cancellation channel, start a goroutine that will gracefully terminate the gRPC server when
	// that channel is closed or receives a `true` value.
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

	return port, done, nil
}
