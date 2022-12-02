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

package main

import (
	"fmt"
	"log"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	secretsrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
	"google.golang.org/grpc"
)

func main() {
	prov := NewCloudSecretPlugin()

	// Fire up a gRPC server, letting the kernel choose a free port for us.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			secretsrpc.RegisterSecretsProviderServer(srv, prov)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	// The secret provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", handle.Port)

	// Finally, wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
