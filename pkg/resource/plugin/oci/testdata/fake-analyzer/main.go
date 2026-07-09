// Copyright 2026, Pulumi Corporation.
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

// A minimal policy pack analyzer used by the OCI launcher integration test.
// Serves the Analyzer gRPC service on PULUMI_POLICY_PORT.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type server struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (s *server) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (s *server) GetAnalyzerInfo(ctx context.Context, _ *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{Name: "oci-integration-pack", Version: "0.0.1"}, nil
}

func (s *server) GetPluginInfo(ctx context.Context, _ *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "0.0.1"}, nil
}

func (s *server) Analyze(ctx context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.AnalyzeResponse, error) {
	return &pulumirpc.AnalyzeResponse{
		Diagnostics: []*pulumirpc.AnalyzeDiagnostic{{
			PolicyName:       "always-fails",
			PolicyPackName:   "oci-integration-pack",
			Description:      "proves the pack ran from its container image",
			Message:          "ran-in-container",
			EnforcementLevel: pulumirpc.EnforcementLevel_MANDATORY,
		}},
	}, nil
}

func main() {
	port := os.Getenv("PULUMI_POLICY_PORT")
	if port == "" {
		port = "0"
	}
	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(srv, &server{})
	// Announce the port (the plugin contract) even though launched packs are
	// dialed by retry, so packs that pick their own port stay discoverable.
	fmt.Printf("%d\n", lis.Addr().(*net.TCPAddr).Port)
	if err := srv.Serve(lis); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
