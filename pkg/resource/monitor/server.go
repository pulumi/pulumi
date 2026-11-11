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
)

// Server is a typed version of [pulumirpc.ResourceMonitorServer].
type Server interface {
	GetDeploymentInfo(context.Context) (*pulumirpc.DeploymentInfo, error)
	SupportsFeature(context.Context, *pulumirpc.SupportsFeatureRequest) (*pulumirpc.SupportsFeatureResponse, error)
	Invoke(context.Context, *pulumirpc.ResourceInvokeRequest) (*pulumirpc.ResourceInvokeResponse, error)
	Call(context.Context, *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)
	ReadResource(context.Context, *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error)
	ExistsResource(context.Context, *pulumirpc.ExistsResourceRequest) (*pulumirpc.ExistsResourceResponse, error)
	RegisterResource(context.Context, *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)
	RegisterResourceOutputs(context.Context, *pulumirpc.RegisterResourceOutputsRequest) error
	RegisterStackTransform(context.Context, *pulumirpc.Callback) error
	RegisterStackInvokeTransform(context.Context, *pulumirpc.Callback) error
	RegisterResourceHook(context.Context, *pulumirpc.RegisterResourceHookRequest) error
	RegisterErrorHook(context.Context, *pulumirpc.RegisterErrorHookRequest) error
	RegisterPackage(context.Context, *pulumirpc.RegisterPackageRequest) (*pulumirpc.RegisterPackageResponse, error)
	SignalAndWaitForShutdown(context.Context) error
}
