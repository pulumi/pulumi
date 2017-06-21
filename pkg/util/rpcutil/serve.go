// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Serve creates a new gRPC server, calls out to the supplied registration functions to bind interfaces, and then
// listens on the supplied TCP port.  If the caller wishes for the kernel to choose a free port automatically, pass 0 as
// the port number.  The return values are: the chosen port (the same as supplied if non-0), a channel that may
// eventually return an error, and an error, in case something went wrong.  The channel is non-nil and waits until
// the server is finished, in the case of a successful launch of the RPC server.
func Serve(port int, cancel chan bool, registers []func(*grpc.Server) error) (int, chan error, error) {
	// Listen on a TCP port, but let the kernel choose a free port for us.
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return port, nil, errors.Errorf("failed to listen on TCP port ':%v': %v", port, err)
	}

	// Now new up a gRPC server and register any RPC interfaces the caller wants.
	srv := grpc.NewServer()
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

	// Now register some signals to gracefully terminate the program upon request.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		if cancel == nil {
			// make a channel that will never resolve.
			cancel = make(chan bool)
		}

		for {
			var stop bool
			select {
			case <-sigs:
				stop = true
			case c := <-cancel:
				stop = c
			}
			if stop {
				srv.GracefulStop()
			}
		}
	}()

	// Finally, serve; this returns only once the server shuts down (e.g., due to a signal).
	done := make(chan error)
	go func() {
		if err := srv.Serve(lis); err != nil &&
			!strings.HasSuffix(err.Error(), "use of closed network connection") {
			done <- errors.Errorf("stopped serving: %v", err)
		} else {
			done <- nil // send a signal so caller knows we're done, even though it's nil.
		}
		close(done)
	}()

	return port, done, nil
}
