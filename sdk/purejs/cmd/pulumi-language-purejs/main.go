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

// pulumi-language-purejs runs `runtime: purejs` policy packs: a pack is one self-contained JavaScript bundle, which
// this host evaluates with goja, so the CLI can enforce it without Node.js or npm. The CLI already picks
// pulumi-language-<runtime> for a pack's runtime:
//
//   - InstallDependencies has nothing to do, so installing a pack is downloading and extracting it;
//   - RunPlugin loads the bundle and serves it as an Analyzer from this process, the service a Node.js pack serves.
//
// The host depends only on the gRPC protocol and goja, not on the SDK's Go packages, to keep the binary small.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func main() {
	srv, lis, err := listen(func(s *grpc.Server) { pulumirpc.RegisterLanguageRuntimeServer(s, &host{}) })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d\n", port(lis))
	if err := srv.Serve(lis); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// listen returns a gRPC server with the given services and the health service, and a free local port to serve on.
func listen(register func(*grpc.Server)) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	s := grpc.NewServer(grpc.MaxRecvMsgSize(400 * 1024 * 1024))
	register(s)
	hs := health.NewServer()
	for name := range s.GetServiceInfo() {
		hs.SetServingStatus(name, healthgrpc.HealthCheckResponse_SERVING)
	}
	healthgrpc.RegisterHealthServer(s, hs)
	return s, lis, nil
}

func port(lis net.Listener) int { return lis.Addr().(*net.TCPAddr).Port }

// verbose reports whether the engine asked for verbose logs (-v=5 or more), in which case console output from
// policies is written to stderr.
var verbose = sync.OnceValue(func() bool {
	for _, arg := range os.Args[1:] {
		arg = strings.TrimLeft(arg, "-")
		if v, ok := strings.CutPrefix(arg, "v="); ok {
			n, err := strconv.Atoi(v)
			return err == nil && n >= 5
		}
	}
	return false
})

type host struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
}

func (h *host) Handshake(
	context.Context, *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (h *host) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (h *host) GetRequiredPlugins(
	context.Context, *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (h *host) GetRequiredPackages(
	context.Context, *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

// InstallDependencies has nothing to do: a purejs pack is its bundle.
func (h *host) InstallDependencies(
	*pulumirpc.InstallDependenciesRequest, pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

// RunPlugin starts an Analyzer server for the pack and streams its port as the plugin's stdout. The engine stops the
// plugin by cancelling the stream.
func (h *host) RunPlugin(req *pulumirpc.RunPluginRequest, stream pulumirpc.LanguageRuntime_RunPluginServer) error {
	if req.GetKind() != "analyzer" {
		return fmt.Errorf("purejs only runs policy packs, not %q plugins", req.GetKind())
	}
	dir := req.GetInfo().GetProgramDirectory()
	if dir == "" {
		dir = req.GetPwd()
	}
	pk, err := loadPack(dir)
	if err != nil {
		return err
	}

	srv, lis, err := listen(func(s *grpc.Server) { pulumirpc.RegisterAnalyzerServer(s, &analyzer{pack: pk}) })
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(lis) }()
	defer srv.Stop()

	if err := stream.Send(&pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{Stdout: []byte(fmt.Sprintf("%d\n", port(lis)))},
	}); err != nil {
		return err
	}
	select {
	case <-stream.Context().Done():
		return nil
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "purejs analyzer for %s: %v\n", dir, err)
		}
	}
	return stream.Send(&pulumirpc.RunPluginResponse{Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: 0}})
}
