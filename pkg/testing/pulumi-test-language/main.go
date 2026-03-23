// Copyright 2016, Pulumi Corporation.
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
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func main() {
	// Initialize OTel tracing if the parent process provided an endpoint.
	if otelEndpoint := os.Getenv("PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT"); otelEndpoint != "" {
		if err := cmdutil.InitOtelTracing("pulumi-test-language", otelEndpoint); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to initialize OTel tracing: %v\n", err)
		}
		defer cmdutil.CloseOtelTracing()
	}

	ctx := context.Background()
	server, err := Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.WriteString(server.Address())
	os.Stdout.Close()

	err = server.Done()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
