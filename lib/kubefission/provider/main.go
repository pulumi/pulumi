// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/rpcutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"google.golang.org/grpc"
)

func main() {
	// Initialize loggers before going any further.
	// TODO: consider parsing flags and letting the Lumi harness propagate them.
	cmdutil.InitLogging(false, 0)

	// Fire up a gRPC server, letting the kernel choose a free port for us.
	port, done, err := rpcutil.Serve(0, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			prov, err := NewProvider()
			if err != nil {
				return fmt.Errorf("failed to create Kube-Fission resource provider: %v", err)
			}
			lumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(-1)
	}

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", port)

	// Finally, wait for the server to stop serving.
	if err := <-done; err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
	}
}
