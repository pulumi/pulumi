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

package provider

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Tracing is the optional command line flag passed to this provider for configuring a  Zipkin-compatible tracing
// endpoint
var tracing string

// Main is the typical entrypoint for a resource provider plugin.  Using it isn't required but can cut down
// significantly on the amount of boilerplate necessary to fire up a new resource provider.
func Main(name string, provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	return MainWithContext(context.Background(), name, provMaker)
}

// MainWithContext is the same as Main but it accepts a context so it can be cancelled.
func MainWithContext(ctx context.Context, name string, provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Parse()

	// Initialize loggers before going any further.
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing(name, name, tracing)

	// Read the non-flags args and connect to the engine.
	var cancelChannel chan bool
	args := flag.Args()
	var host *HostClient
	if len(args) == 0 {
		// Start the provider in Attach mode
	} else if len(args) == 1 {
		var err error
		host, err = NewHostClient(args[0])
		if err != nil {
			return fmt.Errorf("fatal: could not connect to host RPC: %w", err)
		}

		// If we have a host cancel our cancellation context if it fails the healthcheck
		ctx, cancel := context.WithCancel(ctx)
		// map the context Done channel to the rpcutil boolean cancel channel
		cancelChannel = make(chan bool)
		go func() {
			<-ctx.Done()
			close(cancelChannel)
		}()
		err = rpcutil.Healthcheck(ctx, args[0], 5*time.Minute, cancel)
		if err != nil {
			return fmt.Errorf("could not start health check host RPC server: %w", err)
		}
	} else {
		return errors.New("fatal: could not connect to host RPC; missing argument")
	}

	// Fire up a gRPC server, letting the kernel choose a free port for us.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			prov, proverr := provMaker(host)
			if proverr != nil {
				return fmt.Errorf("failed to create resource provider: %v", proverr)
			}
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", handle.Port)

	// Finally, wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	return nil
}
