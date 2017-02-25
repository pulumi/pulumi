// Copyright 2016 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
)

func main() {
	// Listen on a TCP port, but let the kernel choose a free port for us.
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: failed to listen on TCP port ':0': %v\n", err)
		os.Exit(-1)
	}

	// Now new up a gRPC server and register the resource provider implementation.
	srv := grpc.NewServer()
	prov, err := NewProvider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: failed to create AWS resource provider: %v\n", err)
		os.Exit(-1)
	}
	cocorpc.RegisterResourceProviderServer(srv, prov)
	reflection.Register(srv)

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.  To do
	// that, we must retrieve the port chosen by the kernel, by accessing the underlying TCP listener/address.
	tcpl := lis.(*net.TCPListener)
	tcpa := tcpl.Addr().(*net.TCPAddr)
	fmt.Printf("%v\n", strconv.Itoa(tcpa.Port))

	// Now register some signals to gracefully terminate the program upon request.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		<-sigs
		srv.Stop()
	}()

	// Finally, serve; this returns only once the server shuts down (e.g., due to a signal).
	if err := srv.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: stopped serving: %v\n", err)
		os.Exit(-1)
	}
}
