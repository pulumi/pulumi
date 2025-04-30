// Copyright 2016-2020, Pulumi Corporation.
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
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func Import(u UpdateInfo, ctx *Context, opts UpdateOptions, imports []deploy.Import,
	dryRun bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "import", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, []UpdateInfo{u})
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	return update(ctx, info, &deploymentOptions{
		UpdateOptions:   opts,
		SourceFunc:      newRefreshSource,
		Events:          emitter,
		Diag:            newEventSink(emitter, false),
		StatusDiag:      newEventSink(emitter, true),
		debugTraceMutex: &sync.Mutex{},
		isImport:        true,
		imports:         imports,
		DryRun:          dryRun,
	})
}
