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

package sdk

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PolicyProxy implements the pulumirpc.AnalyzerServer interface by proxying all requests to a real policy pack. This is
// used by node and python language hosts to proxy new style engine requests for policy packs (using RunPlugin) to
// historical policy pack libraries that expected to be called with a informational environment variables set.
//
// Nothing new should use this type! New policy pack implementations should implement StackConfigure and Handshake
// internally as part of the analyzer plugin process.
type PolicyProxy struct {
	pulumirpc.UnsafeAnalyzerServer

	stackConfiguration *promise.CompletionSource[*pulumirpc.AnalyzerStackConfigureRequest]
	client             *promise.CompletionSource[pulumirpc.AnalyzerClient]

	port int

	stdout  io.Writer
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
}

// NewPolicyProxy starts a new grpc server implementing the Analyzer service that proxies all requests to the actual
// policy pack process. The only request that is handled directly is Handshake and StackConfiguration, which are used to
// start up the actual policy pack process. For compatibility if we get a call to GetAnalyzerInfo without having seen a
// call for StackConfiguration we will start the policy pack process with no configuration.
func NewPolicyProxy(ctx context.Context, stdout io.Writer) (*PolicyProxy, io.Writer, error) {
	stdoutR, stdoutW := io.Pipe()
	analyzer := &PolicyProxy{
		stackConfiguration: &promise.CompletionSource[*pulumirpc.AnalyzerStackConfigureRequest]{},
		client:             &promise.CompletionSource[pulumirpc.AnalyzerClient]{},

		stdout:  stdout,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
	}

	var cancel chan bool
	done := ctx.Done()
	if done != nil {
		cancel = make(chan bool, 1)
		go func() {
			<-done
			cancel <- true
		}()
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, analyzer)
			return nil
		},
	})
	if err != nil {
		return nil, nil, err
	}
	analyzer.port = handle.Port
	logging.V(9).Infof("Started policy proxy on port %d", analyzer.port)

	return analyzer, stdoutW, nil
}

func (p *PolicyProxy) AwaitConfiguration(ctx context.Context) (*pulumirpc.AnalyzerStackConfigureRequest, error) {
	logging.V(9).Infof("Awaiting policy pack configuration on port %d", p.port)
	fmt.Fprintf(p.stdout, "%d\n", p.port)
	config, err := p.stackConfiguration.Promise().Result(ctx)
	logging.V(9).Infof("Awaited policy pack configuration %v", config)
	return config, err
}

// Attach waits for the subcommand to run and tries to proxy the grpc connection to it.
func (p *PolicyProxy) Attach(ctx context.Context, cmd *exec.Cmd) error {
	// We need to wait for cmd to exit and close off stdout when it does so the fscanf doesn't block forever.
	exit := &promise.CompletionSource[struct{}]{}
	go func() {
		err := cmd.Wait()
		if err == nil {
			exit.Fulfill(struct{}{})
		} else {
			exit.Reject(err)
		}
		p.stdoutW.Close()
		p.stdoutR.Close()
	}()

	logging.V(9).Infof("Waiting to attach to policy pack")
	// Read the port number from the subprocess's stdout.
	var port int
	_, err := fmt.Fscanf(p.stdoutR, "%d\n", &port)
	if err != nil {
		err = fmt.Errorf("could not read policy pack port: %w", err)
		p.client.Reject(err)
		return err
	}
	logging.V(9).Infof("Attaching to policy pack on port %d", port)

	// Proxy everything else to the normal stdout.
	go func() {
		_, err := io.Copy(p.stdout, p.stdoutR)
		if err != nil {
			fmt.Fprintf(p.stdout, "error copying stdout: %v\n", err)
		}
	}()

	// Attach to the subprocess over gRPC.
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		err = fmt.Errorf("dial policy pack: %w", err)
		p.client.Reject(err)
		return err
	}

	client := pulumirpc.NewAnalyzerClient(conn)

	// Now that we're attached to the subprocess forward it the stack configuration we got earlier.
	stkcfg, err, has := p.stackConfiguration.Promise().TryResult()
	contract.AssertNoErrorf(err, "stackConfiguration promise should never be rejected")
	if has {
		_, err := client.ConfigureStack(ctx, stkcfg)
		if err != nil {
			return fmt.Errorf("policy pack configuration failed: %w", err)
		}
	}

	p.client.Fulfill(client)
	_, err = exit.Promise().Result(ctx)
	return err
}

func (p *PolicyProxy) ConfigureStack(
	ctx context.Context,
	req *pulumirpc.AnalyzerStackConfigureRequest,
) (*pulumirpc.AnalyzerStackConfigureResponse, error) {
	p.stackConfiguration.Fulfill(req)
	return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
}

func (p *PolicyProxy) Handshake(
	ctx context.Context,
	req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (p *PolicyProxy) awaitClient(ctx context.Context) (pulumirpc.AnalyzerClient, error) {
	// The engine sometimes calls others methods before calling StackConfiguration, for example for `policy
	// publish`, in this case just report no configuration.
	p.stackConfiguration.Fulfill(nil)
	logging.V(9).Infof("Awaiting policy pack client")
	client, err := p.client.Promise().Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("policy pack not started: %w", err)
	}
	return client, nil
}

func (p *PolicyProxy) Analyze(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.Analyze(ctx, req)
}

func (p *PolicyProxy) AnalyzeStack(
	ctx context.Context, req *pulumirpc.AnalyzeStackRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.AnalyzeStack(ctx, req)
}

func (p *PolicyProxy) Remediate(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.RemediateResponse, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.Remediate(ctx, req)
}

func (p *PolicyProxy) GetAnalyzerInfo(
	ctx context.Context,
	req *emptypb.Empty,
) (*pulumirpc.AnalyzerInfo, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetAnalyzerInfo(ctx, req)
}

func (p *PolicyProxy) GetPluginInfo(
	ctx context.Context,
	req *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	logging.V(9).Infof("GetPluginInfo")
	client, err := p.awaitClient(ctx)
	logging.V(9).Infof("GetPluginInfo: awaitClient returned err=%v", err)
	if err != nil {
		return nil, err
	}
	info, err := client.GetPluginInfo(ctx, req)
	logging.V(9).Infof("GetPluginInfo: client.GetPluginInfo returned info=%v, err=%v", info, err)
	return info, err
}

func (p *PolicyProxy) Configure(ctx context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*emptypb.Empty, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.Configure(ctx, req)
}

func (p *PolicyProxy) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	client, err := p.awaitClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.Cancel(ctx, req)
}
