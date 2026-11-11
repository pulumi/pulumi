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

// This file is written by hand. Its source is proto/go/provider_check_config.go; proto/generate.sh copies it next to
// the generated code and patches provider_grpc.pb.go to match.
//
// The CheckConfig RPC returns a CheckConfigResponse, which is wire compatible with the CheckResponse it used to
// return. So that existing Go code keeps compiling, the Go bindings expose the RPC as two methods: CheckConfig, with
// the CheckResponse signature it has always had, and CheckConfig2, with the types from the protocol definition.

package pulumirpc

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

func (c *resourceProviderClient) CheckConfig(
	ctx context.Context, in *CheckRequest, opts ...grpc.CallOption,
) (*CheckResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckResponse)
	err := c.cc.Invoke(ctx, ResourceProvider_CheckConfig_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (UnimplementedResourceProviderServer) CheckConfig(context.Context, *CheckRequest) (*CheckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckConfig not implemented")
}

// _ResourceProvider_CheckConfig_Dispatch serves the CheckConfig RPC from whichever of the two Go methods the server
// implements, preferring CheckConfig2.
func _ResourceProvider_CheckConfig_Dispatch(
	srv ResourceProviderServer, ctx context.Context, in *CheckRequest,
) (*CheckConfigResponse, error) {
	out, err := srv.CheckConfig2(ctx, in)
	if status.Code(err) != codes.Unimplemented {
		return out, err
	}

	old, err := srv.CheckConfig(ctx, in)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, nil
	}
	resp := &CheckConfigResponse{Inputs: old.GetInputs()}
	for _, f := range old.GetFailures() {
		resp.Failures = append(resp.Failures, &CheckConfigFailure{
			Property: f.GetProperty(),
			Reason:   f.GetReason(),
		})
	}
	return resp, nil
}
