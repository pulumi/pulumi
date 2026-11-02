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

package monitor

import (
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AsGRPC projects [Server] into a gRPC compatible monitor.
func AsGRPC(s Server) pulumirpc.ResourceMonitorServer {
	return &grpcServer{s: s}
}

// A bridge from [Server] to [pulumirpc.ResourceMonitorServer]
type grpcServer struct {
	pulumirpc.UnimplementedResourceMonitorServer
	s Server
}

func (s *grpcServer) GetDeploymentInfo(ctx context.Context, _ *emptypb.Empty) (*pulumirpc.DeploymentInfo, error) {
	return s.s.GetDeploymentInfo(ctx)
}

func (s *grpcServer) SupportsFeature(
	ctx context.Context, req *pulumirpc.SupportsFeatureRequest,
) (*pulumirpc.SupportsFeatureResponse, error) {
	return s.s.SupportsFeature(ctx, req)
}

func (s *grpcServer) Invoke(
	ctx context.Context, req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.ResourceInvokeResponse, error) {
	return s.s.Invoke(ctx, req)
}

func (s *grpcServer) Call(ctx context.Context, req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error) {
	return s.s.Call(ctx, req)
}

func (s *grpcServer) ReadResource(
	ctx context.Context, req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	return s.s.ReadResource(ctx, req)
}

func (s *grpcServer) ExistsResource(
	ctx context.Context, req *pulumirpc.ExistsResourceRequest,
) (*pulumirpc.ExistsResourceResponse, error) {
	return s.s.ExistsResource(ctx, req)
}

func (s *grpcServer) RegisterResource(
	ctx context.Context, req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	return s.s.RegisterResource(ctx, req)
}

func (s *grpcServer) RegisterResourceOutputs(
	ctx context.Context, req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.RegisterResourceOutputs(ctx, req)
}

func (s *grpcServer) RegisterStackTransform(ctx context.Context, req *pulumirpc.Callback) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.RegisterStackTransform(ctx, req)
}

func (s *grpcServer) RegisterStackInvokeTransform(
	ctx context.Context, req *pulumirpc.Callback,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.RegisterStackInvokeTransform(ctx, req)
}

func (s *grpcServer) RegisterResourceHook(
	ctx context.Context, req *pulumirpc.RegisterResourceHookRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.RegisterResourceHook(ctx, req)
}

func (s *grpcServer) RegisterErrorHook(
	ctx context.Context, req *pulumirpc.RegisterErrorHookRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.RegisterErrorHook(ctx, req)
}

func (s *grpcServer) RegisterPackage(
	ctx context.Context, req *pulumirpc.RegisterPackageRequest,
) (*pulumirpc.RegisterPackageResponse, error) {
	return s.s.RegisterPackage(ctx, req)
}

func (s *grpcServer) SignalAndWaitForShutdown(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.s.SignalAndWaitForShutdown(ctx)
}
