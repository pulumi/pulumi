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

package rpcutil

import (
	"context"
	"io"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type outputServer struct {
	pulumirpc.UnimplementedOutputServer

	stdout     io.WriteCloser
	stderr     io.WriteCloser
	isTerminal bool
}

func NewOutputServer(stdout, stderr io.WriteCloser, isTerminal bool) pulumirpc.OutputServer {
	return &outputServer{
		stdout:     stdout,
		stderr:     stderr,
		isTerminal: isTerminal,
	}
}

func (s *outputServer) Write(ctx context.Context, req *pulumirpc.WriteRequest) (*emptypb.Empty, error) {
	var err error
	switch data := req.Data.(type) {
	case *pulumirpc.WriteRequest_Stdout:
		_, err = s.stdout.Write(data.Stdout)
	case *pulumirpc.WriteRequest_Stderr:
		_, err = s.stderr.Write(data.Stderr)
	}
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, err
}

func (s *outputServer) Close(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if s.stdout == nil {
		contract.Assertf(s.stderr == nil, "stderr should be nil if stdout is nil")
		return &emptypb.Empty{}, nil
	}

	oerr := s.stdout.Close()
	eerr := s.stderr.Close()
	err := multierror.Append(oerr, eerr).ErrorOrNil()
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *outputServer) GetCapabilities(
	ctx context.Context, req *emptypb.Empty,
) (*pulumirpc.GetCapabilitiesResponse, error) {
	return &pulumirpc.GetCapabilitiesResponse{
		IsTerminal: s.isTerminal,
	}, nil
}

func OutputRegistration(l pulumirpc.OutputServer) func(*grpc.Server) {
	return func(srv *grpc.Server) {
		pulumirpc.RegisterOutputServer(srv, l)
	}
}
