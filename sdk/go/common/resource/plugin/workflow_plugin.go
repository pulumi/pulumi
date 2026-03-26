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

package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type workflowPlugin struct {
	path      string
	plug      *Plugin
	clientRaw pulumirpc.WorkflowEvaluatorClient
}

// NewWorkflow launches a workflow evaluator plugin and establishes a gRPC connection.
//
// The workflow source follows normal plugin conventions:
// - path to an executable binary, or
// - path to a directory (or plugin path) containing PulumiPlugin.yaml.
func NewWorkflow(host Host, ctx *Context, source string) (Workflow, error) {
	pluginPath, err := resolveWorkflowPluginPath(host, ctx, source)
	if err != nil {
		return nil, err
	}

	absPluginPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workflow plugin path: %w", err)
	}

	prefix := fmt.Sprintf("%v (workflow)", filepath.Base(absPluginPath))
	pluginDir := absPluginPath

	handshake := func(
		ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
	) (*pulumirpc.WorkflowHandshakeResponse, error) {
		client := pulumirpc.NewWorkflowEvaluatorClient(conn)
		if stat, err := os.Stat(bin); err == nil && !stat.IsDir() {
			pluginDir = filepath.Dir(bin)
		}
		res, err := client.Handshake(ctx, &pulumirpc.WorkflowHandshakeRequest{
			EngineAddress:    host.ServerAddr(),
			RootDirectory:    &pluginDir,
			ProgramDirectory: &pluginDir,
		})
		if err != nil {
			rpcStatus, ok := status.FromError(err)
			if ok && rpcStatus.Code() == codes.Unimplemented {
				logging.V(7).Infof("Workflow handshake not supported by '%v'", bin)
				return &pulumirpc.WorkflowHandshakeResponse{}, nil
			}
			return nil, err
		}
		logging.V(7).Infof("Workflow handshake succeeded [%v]", bin)
		return res, nil
	}

	plug, _, err := newPlugin(
		ctx,
		ctx.Pwd,
		absPluginPath,
		prefix,
		apitype.WorkflowPlugin,
		nil,
		env.Global(),
		handshake,
		workflowPluginDialOptions(ctx, absPluginPath),
		host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: filepath.Base(absPluginPath)}),
	)
	if err != nil {
		return nil, err
	}

	contract.Assertf(plug != nil, "unexpected nil workflow plugin for %s", absPluginPath)

	return &workflowPlugin{
		path:      absPluginPath,
		plug:      plug,
		clientRaw: pulumirpc.NewWorkflowEvaluatorClient(plug.Conn),
	}, nil
}

func resolveWorkflowPluginPath(host Host, ctx *Context, source string) (string, error) {
	if IsLocalPluginPath(ctx.baseContext, source) {
		return source, nil
	}

	spec, err := workspace.NewPluginDescriptor(ctx.baseContext, source, apitype.WorkflowPlugin, nil, "", nil)
	if err != nil {
		return "", err
	}

	path, err := workspace.GetPluginPath(ctx.baseContext, ctx.Diag, spec, host.GetProjectPlugins())
	if err != nil {
		return "", err
	}
	contract.Assertf(path != "", "unexpected empty path for workflow source %s", source)
	return path, nil
}

func workflowPluginDialOptions(ctx *Context, path string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.TracingInterceptorDialOptions(otgrpc.SpanDecorator(decorateProviderSpans)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]any{
			"mode": "client",
			"kind": "workflow",
			"path": path,
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

func (p *workflowPlugin) Close() error {
	if p.plug == nil {
		return nil
	}
	return p.plug.Close()
}

func (p *workflowPlugin) Handshake(
	ctx context.Context, req *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return p.clientRaw.Handshake(ctx, req)
}

func (p *workflowPlugin) GetPackageInfo(
	ctx context.Context, req *pulumirpc.GetPackageInfoRequest,
) (*pulumirpc.GetPackageInfoResponse, error) {
	label := fmt.Sprintf("Workflow[%s, %p].GetPackageInfo", p.path, p)
	logging.V(7).Infof("%s executing", label)
	resp, err := p.clientRaw.GetPackageInfo(ctx, req)
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("%s rpc error `%s`: `%s`", label, rpcError.Code(), rpcError.Message())
		return nil, err
	}
	return resp, nil
}

func (p *workflowPlugin) GetGraphs(
	ctx context.Context, req *pulumirpc.GetGraphsRequest,
) (*pulumirpc.GetGraphsResponse, error) {
	return p.clientRaw.GetGraphs(ctx, req)
}

func (p *workflowPlugin) GetGraph(
	ctx context.Context, req *pulumirpc.GetGraphRequest,
) (*pulumirpc.GetGraphResponse, error) {
	return p.clientRaw.GetGraph(ctx, req)
}

func (p *workflowPlugin) GetTriggers(
	ctx context.Context, req *pulumirpc.GetTriggersRequest,
) (*pulumirpc.GetTriggersResponse, error) {
	return p.clientRaw.GetTriggers(ctx, req)
}

func (p *workflowPlugin) GetTrigger(
	ctx context.Context, req *pulumirpc.GetTriggerRequest,
) (*pulumirpc.GetTriggerResponse, error) {
	return p.clientRaw.GetTrigger(ctx, req)
}

func (p *workflowPlugin) GetJobs(
	ctx context.Context, req *pulumirpc.GetJobsRequest,
) (*pulumirpc.GetJobsResponse, error) {
	return p.clientRaw.GetJobs(ctx, req)
}

func (p *workflowPlugin) GetJob(
	ctx context.Context, req *pulumirpc.GetJobRequest,
) (*pulumirpc.GetJobResponse, error) {
	return p.clientRaw.GetJob(ctx, req)
}

func (p *workflowPlugin) GenerateGraph(
	ctx context.Context, req *pulumirpc.GenerateGraphRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return p.clientRaw.GenerateGraph(ctx, req)
}

func (p *workflowPlugin) GenerateJob(
	ctx context.Context, req *pulumirpc.GenerateJobRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return p.clientRaw.GenerateJob(ctx, req)
}

func (p *workflowPlugin) RunTriggerMock(
	ctx context.Context, req *pulumirpc.RunTriggerMockRequest,
) (*pulumirpc.RunTriggerMockResponse, error) {
	return p.clientRaw.RunTriggerMock(ctx, req)
}

func (p *workflowPlugin) RunFilter(
	ctx context.Context, req *pulumirpc.RunFilterRequest,
) (*pulumirpc.RunFilterResponse, error) {
	return p.clientRaw.RunFilter(ctx, req)
}

func (p *workflowPlugin) RunStep(
	ctx context.Context, req *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	return p.clientRaw.RunStep(ctx, req)
}

func (p *workflowPlugin) RunOnError(
	ctx context.Context, req *pulumirpc.RunOnErrorRequest,
) (*pulumirpc.RunOnErrorResponse, error) {
	return p.clientRaw.RunOnError(ctx, req)
}

func (p *workflowPlugin) ResolveJobResult(
	ctx context.Context, req *pulumirpc.ResolveJobResultRequest,
) (*pulumirpc.ResolveJobResultResponse, error) {
	return p.clientRaw.ResolveJobResult(ctx, req)
}
