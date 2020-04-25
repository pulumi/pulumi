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

package engine

import (
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

func Import(u UpdateInfo, ctx *Context, opts UpdateOptions, imports []deploy.Import,
	dryRun bool) (ResourceChanges, result.Result) {

	contract.Require(u != nil, "u")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u, "import", ctx.ParentSpan)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer emitter.Close()

	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newRefreshSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		isImport:      true,
		imports:       imports,
	}, dryRun)
}

func newImportSource(client deploy.BackendClient, opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	// Like Update, we need to gather the set of plugins necessary to refresh everything in the snapshot.
	// Unlike Update, we don't actually run the user's program so we only need the set of plugins described
	// in the snapshot.
	plugins, err := gatherPluginsFromSnapshot(plugctx, target)
	if err != nil {
		return nil, err
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	if err := ensurePluginsAreInstalled(plugins); err != nil {
		logging.V(7).Infof("newRefreshSource(): failed to install missing plugins: %v", err)
	}

	// Return nil. Imports don't require a source.
	return nil, nil
}
