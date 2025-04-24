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

package auto

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/opthistory"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremotedestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremotepreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremoterefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremoteup"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// RemoteStack is an isolated, independently configurable instance of a Pulumi program that is
// operated on remotely (up/preview/refresh/destroy).
type RemoteStack struct {
	stack Stack
}

// Name returns the stack name.
func (s *RemoteStack) Name() string {
	return s.stack.Name()
}

// Preview preforms a dry-run update to a stack, returning pending changes.
// https://www.pulumi.com/docs/cli/commands/pulumi_preview/
// This operation runs remotely.
func (s *RemoteStack) Preview(ctx context.Context, opts ...optremotepreview.Option) (PreviewResult, error) {
	preOpts := &optremotepreview.Options{}
	for _, o := range opts {
		o.ApplyOption(preOpts)
	}

	implOpts := []optpreview.Option{}
	if preOpts.ProgressStreams != nil {
		implOpts = append(implOpts, optpreview.ProgressStreams(preOpts.ProgressStreams...))
	}
	if preOpts.ErrorProgressStreams != nil {
		implOpts = append(implOpts, optpreview.ErrorProgressStreams(preOpts.ErrorProgressStreams...))
	}
	if preOpts.EventStreams != nil {
		implOpts = append(implOpts, optpreview.EventStreams(preOpts.EventStreams...))
	}

	return s.stack.Preview(ctx, implOpts...)
}

// Up creates or updates the resources in a stack by executing the program in the Workspace.
// https://www.pulumi.com/docs/cli/commands/pulumi_up/
// This operation runs remotely.
func (s *RemoteStack) Up(ctx context.Context, opts ...optremoteup.Option) (UpResult, error) {
	upOpts := &optremoteup.Options{}
	for _, o := range opts {
		o.ApplyOption(upOpts)
	}

	implOpts := []optup.Option{}
	if upOpts.ProgressStreams != nil {
		implOpts = append(implOpts, optup.ProgressStreams(upOpts.ProgressStreams...))
	}
	if upOpts.ErrorProgressStreams != nil {
		implOpts = append(implOpts, optup.ErrorProgressStreams(upOpts.ErrorProgressStreams...))
	}
	if upOpts.EventStreams != nil {
		implOpts = append(implOpts, optup.EventStreams(upOpts.EventStreams...))
	}

	return s.stack.Up(ctx, implOpts...)
}

// Refresh compares the current stack’s resource state with the state known to exist in the actual
// cloud provider. Any such changes are adopted into the current stack.
// This operation runs remotely.
func (s *RemoteStack) Refresh(ctx context.Context, opts ...optremoterefresh.Option) (RefreshResult, error) {
	refreshOpts := &optremoterefresh.Options{}
	for _, o := range opts {
		o.ApplyOption(refreshOpts)
	}

	implOpts := []optrefresh.Option{}
	if refreshOpts.ProgressStreams != nil {
		implOpts = append(implOpts, optrefresh.ProgressStreams(refreshOpts.ProgressStreams...))
	}
	if refreshOpts.ErrorProgressStreams != nil {
		implOpts = append(implOpts, optrefresh.ErrorProgressStreams(refreshOpts.ErrorProgressStreams...))
	}
	if refreshOpts.EventStreams != nil {
		implOpts = append(implOpts, optrefresh.EventStreams(refreshOpts.EventStreams...))
	}

	return s.stack.Refresh(ctx, implOpts...)
}

// Destroy deletes all resources in a stack, leaving all history and configuration intact.
// This operation runs remotely.
func (s *RemoteStack) Destroy(ctx context.Context, opts ...optremotedestroy.Option) (DestroyResult, error) {
	destroyOpts := &optremotedestroy.Options{}
	for _, o := range opts {
		o.ApplyOption(destroyOpts)
	}

	implOpts := []optdestroy.Option{}
	if destroyOpts.ProgressStreams != nil {
		implOpts = append(implOpts, optdestroy.ProgressStreams(destroyOpts.ProgressStreams...))
	}
	if destroyOpts.ErrorProgressStreams != nil {
		implOpts = append(implOpts, optdestroy.ErrorProgressStreams(destroyOpts.ErrorProgressStreams...))
	}
	if destroyOpts.EventStreams != nil {
		implOpts = append(implOpts, optdestroy.EventStreams(destroyOpts.EventStreams...))
	}

	return s.stack.Destroy(ctx, implOpts...)
}

// Outputs get the current set of Stack outputs from the last Stack.Up().
func (s *RemoteStack) Outputs(ctx context.Context) (OutputMap, error) {
	return s.stack.Workspace().StackOutputs(ctx, s.Name())
}

// History returns a list summarizing all previous and current results from Stack lifecycle operations
// (up/preview/refresh/destroy).
func (s *RemoteStack) History(ctx context.Context, pageSize, page int) ([]UpdateSummary, error) {
	// Note: Find a way to allow options for ShowSecrets(true) that doesn't require loading the project.
	return s.stack.History(ctx, pageSize, page, opthistory.ShowSecrets(false))
}

// Cancel stops a stack's currently running update. It returns an error if no update is currently running.
// Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
// if a resource operation was pending when the update was canceled.
func (s *RemoteStack) Cancel(ctx context.Context) error {
	return s.stack.Cancel(ctx)
}

// Export exports the deployment state of the stack.
// This can be combined with Stack.Import to edit a stack's state (such as recovery from failed deployments).
func (s *RemoteStack) Export(ctx context.Context) (apitype.UntypedDeployment, error) {
	return s.stack.Workspace().ExportStack(ctx, s.Name())
}

// Import imports the specified deployment state into the stack.
// This can be combined with Stack.Export to edit a stack's state (such as recovery from failed deployments).
func (s *RemoteStack) Import(ctx context.Context, state apitype.UntypedDeployment) error {
	return s.stack.Workspace().ImportStack(ctx, s.Name(), state)
}
