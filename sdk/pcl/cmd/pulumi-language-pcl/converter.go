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

package main

import (
	"context"
	"errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// pclConverterHost implements the ConverterServer interface.
type pclConverterHost struct {
	pulumirpc.UnsafeConverterServer

	engineAddress string
}

func newConverterHost(engineAddress string) pulumirpc.ConverterServer {
	return &pclConverterHost{
		engineAddress: engineAddress,
	}
}

func (h *pclConverterHost) ConvertProgram(ctx context.Context, req *pulumirpc.ConvertProgramRequest) (*pulumirpc.ConvertProgramResponse, error) {
	err := fsutil.CopyFile(req.TargetDirectory, req.SourceDirectory, nil)
	if err != nil {
		return nil, err
	}
	return &pulumirpc.ConvertProgramResponse{}, nil
}

func (h *pclConverterHost) ConvertState(context.Context, *pulumirpc.ConvertStateRequest) (*pulumirpc.ConvertStateResponse, error) {
	return nil, errors.New("not implemented")
}
