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

package plugin

import (
	"context"
	"encoding/json"

	"github.com/blang/semver"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	secretsrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
)

type secretsPlugin struct {
	plug   *plugin
	client secretsrpc.SecretsProviderClient
}

func NewSecretsProviderPlugin(host Host, ctx *Context, pwd string,
	name string, version *semver.Version) (SecretsProvider, error) {

	path, err := workspace.GetPluginPath(
		workspace.SecretsPlugin, name, version, host.GetProjectPlugins())
	if err != nil {
		return nil, err
	}

	contract.Assert(path != "")

	plug, err := newPlugin(ctx, pwd, path, name,
		workspace.SecretsPlugin, nil, nil /*env*/, secretsPluginDialOptions(ctx, name))
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil secrets plugin for %s", name)

	return &secretsPlugin{
		plug:   plug,
		client: secretsrpc.NewSecretsProviderClient(plug.Conn),
	}, nil
}

func secretsPluginDialOptions(ctx *Context, name string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(),
		grpc.WithInsecure(),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
			"mode": "client",
			"kind": "language",
		}
		if name != "" {
			metadata["name"] = name
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

func (p *secretsPlugin) Close() error {
	if p.plug == nil {
		return nil
	}
	return p.plug.Close()
}

func (p *secretsPlugin) Initalize(ctx context.Context, args []string, inputs map[string]string) (*Prompt, *json.RawMessage, error) {
	resp, err := p.client.Initialize(ctx, &secretsrpc.InitializeRequest{
		Args:   args,
		Inputs: inputs,
	})
	if err != nil {
		return nil, nil, err
	}
	if resp.Prompt != nil {
		return &Prompt{
			Label:    resp.Prompt.Label,
			Text:     resp.Prompt.Text,
			Error:    resp.Prompt.Error,
			Preserve: resp.Prompt.Preserve,
		}, nil, nil
	}

	var state json.RawMessage
	err = json.Unmarshal([]byte(resp.State), &state)
	if err != nil {
		return nil, nil, err
	}
	return nil, &state, nil
}

func (p *secretsPlugin) Configure(ctx context.Context, state json.RawMessage, inputs map[string]string) (*Prompt, error) {
	state, err := state.MarshalJSON()
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Configure(ctx, &secretsrpc.ConfigureRequest{
		State:  string(state),
		Inputs: inputs,
	})
	if resp.Prompt != nil {
		return &Prompt{
			Label:    resp.Prompt.Label,
			Text:     resp.Prompt.Text,
			Error:    resp.Prompt.Error,
			Preserve: resp.Prompt.Preserve,
		}, nil
	}

	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (p *secretsPlugin) Encrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	resp, err := p.client.Encrypt(ctx, &secretsrpc.EncryptRequest{Plaintexts: plaintexts})
	if err != nil {
		return nil, err
	}
	return resp.Ciphertexts, err
}

func (p *secretsPlugin) Decrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	resp, err := p.client.Decrypt(ctx, &secretsrpc.DecryptRequest{Ciphertexts: ciphertexts})
	if err != nil {
		return nil, err
	}
	return resp.Plaintexts, err
}
