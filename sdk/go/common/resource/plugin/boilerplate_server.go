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

type boilerplateServer struct {
	pulumirpc.UnimplementedBoilerplateServer

	boilerplate Boilerplate
}

func NewBoilerplateServer(boilerplate Boilerplate) pulumirpc.BoilerplateServer {
	return &boilerplateServer{boilerplate: boilerplate}
}

func (c *boilerplateServer) CreatePackage(ctx context.Context,
	req *pulumirpc.CreatePackageRequest,
) (*pulumirpc.CreatePackageResponse, error) {
	_, err := c.boilerplate.CreatePackage(ctx, &CreatePackageRequest{
		Name:   req.Name,
		Config: req.Config,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CreatePackageResponse{}, nil
}
