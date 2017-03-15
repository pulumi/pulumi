// Copyright 2017 Pulumi, Inc. All rights reserved.

package rpcutil

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Serve creates a new gRPC server, calls out to the supplied registration functions to bind interfaces, and then
// listens on the supplied TCP port.  If the caller wishes for the kernel to choose a free port automatically, pass 0 as
// the port number.  The return values are: the chosen port (the same as supplied if non-0), a channel that may
// eventually return an error, and an error, in case something went wrong.  The channel is non-nil and waits until
// the server is finished, in the case of a successful launch of the RPC server.
func Serve(port int, registers []func(*grpc.Server) error) (int, chan error, error) {
	// Listen on a TCP port, but let the kernel choose a free port for us.
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return port, nil, fmt.Errorf("failed to listen on TCP port ':%v': %v", port, err)
	}

	// Now new up a gRPC server and register any RPC interfaces the caller wants.
	srv := grpc.NewServer()
	for _, register := range registers {
		if err := register(srv); err != nil {
			return port, nil, fmt.Errorf("failed to register RPC handler: %v", err)
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
		<-sigs
		srv.Stop()
	}()

	// Finally, serve; this returns only once the server shuts down (e.g., due to a signal).
	done := make(chan error)
	go func() {
		if err := srv.Serve(lis); err != nil {
			done <- fmt.Errorf("stopped serving: %v", err)
		} else {
			done <- nil // send a signal so caller knows we're done, even though it's nil.
		}
		close(done)
	}()

	return port, done, nil
}
