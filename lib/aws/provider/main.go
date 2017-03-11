// Copyright 2016 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/coconut/pkg/util/rpcutil"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"google.golang.org/grpc"
)

func main() {
	// Fire up a gRPC server, letting the kernel choose a free port for us.
	port, done, err := rpcutil.Serve(0, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			prov, err := NewProvider()
			if err != nil {
				return fmt.Errorf("failed to create AWS resource provider: %v", err)
			}
			cocorpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(-1)
	}

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%n\n", port)

	// Finally, wait for the server to stop serving.
	if err := <-done; err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
	}
}
