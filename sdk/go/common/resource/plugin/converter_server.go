// Copyright 2016-2023, Pulumi Corporation.
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

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type converterServer struct {
	pulumirpc.UnsafeConverterServer // opt out of forward compat

	converter Converter
}

func NewConverterServer(converter Converter) pulumirpc.ConverterServer {
	return &converterServer{converter: converter}
}

func (c *converterServer) ConvertState(ctx context.Context,
	req *pulumirpc.ConvertStateRequest,
) (*pulumirpc.ConvertStateResponse, error) {
	resp, err := c.converter.ConvertState(ctx, &ConvertStateRequest{})
	if err != nil {
		return nil, err
	}

	resources := make([]*pulumirpc.ResourceImport, len(resp.Resources))
	for i, resource := range resp.Resources {
		resources[i] = &pulumirpc.ResourceImport{
			Type:              resource.Type,
			Name:              resource.Name,
			Id:                resource.ID,
			Version:           resource.Version,
			PluginDownloadURL: resource.PluginDownloadURL,
		}
	}

	rpcResp := &pulumirpc.ConvertStateResponse{
		Resources: resources,
	}
	return rpcResp, nil
}

func (c *converterServer) ConvertProgram(ctx context.Context,
	req *pulumirpc.ConvertProgramRequest,
) (*pulumirpc.ConvertProgramResponse, error) {
	_, err := c.converter.ConvertProgram(ctx, &ConvertProgramRequest{
		SourceDirectory: req.SourceDirectory,
		TargetDirectory: req.TargetDirectory,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.ConvertProgramResponse{}, nil
}
