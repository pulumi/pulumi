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

package plugin

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

// engineServer wraps an Engine in a gRPC interface.
// engineServer is the server side of the host RPC machinery.
type engineServer struct {
	engine Engine // the wrapped engine
}

func NewEngineServer(engine Engine) pulumirpc.EngineServer {
	return &engineServer{engine: engine}
}

// Log logs a global message in the engine, including errors and warnings.
func (s *engineServer) Log(ctx context.Context, req *pulumirpc.LogRequest) (*empty.Empty, error) {
	var sev diag.Severity
	switch req.Severity {
	case pulumirpc.LogSeverity_DEBUG:
		sev = diag.Debug
	case pulumirpc.LogSeverity_INFO:
		sev = diag.Info
	case pulumirpc.LogSeverity_WARNING:
		sev = diag.Warning
	case pulumirpc.LogSeverity_ERROR:
		sev = diag.Error
	default:
		return nil, errors.Errorf("Unrecognized logging severity: %v", req.Severity)
	}

	if req.Ephemeral {
		s.engine.LogStatus(ctx, sev, resource.URN(req.GetUrn()), req.GetMessage(), req.GetStreamId())
	} else {
		s.engine.Log(ctx, sev, resource.URN(req.GetUrn()), req.GetMessage(), req.GetStreamId())
	}
	return &empty.Empty{}, nil
}

// GetRootResource returns the current root resource's URN, which will serve as the parent of resources that are
// otherwise left unparented.
func (s *engineServer) GetRootResource(ctx context.Context,
	req *pulumirpc.GetRootResourceRequest) (*pulumirpc.GetRootResourceResponse, error) {

	urn, err := s.engine.GetRootResource(ctx)
	if err != nil {
		return nil, err
	}
	return &pulumirpc.GetRootResourceResponse{Urn: string(urn)}, nil
}

// SetRootResources sets the current root resource's URN. Generally only called on startup when the Stack resource is
// registered.
func (s *engineServer) SetRootResource(ctx context.Context,
	req *pulumirpc.SetRootResourceRequest) (*pulumirpc.SetRootResourceResponse, error) {

	if err := s.engine.SetRootResource(ctx, resource.URN(req.GetUrn())); err != nil {
		return nil, err
	}
	return &pulumirpc.SetRootResourceResponse{}, nil
}
