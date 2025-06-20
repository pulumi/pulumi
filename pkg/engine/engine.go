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
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// UpdateInfo handles information common to resource operations (update, preview, destroy, import, refresh).
type UpdateInfo struct {
	// Root is the root directory for this update. This defines the scope for any filesystem resources accessed by this
	// update.
	Root string
	// Project is the project associated with this update. This includes information such as the runtime that will be used
	// to execute the Pulumi program and the program's relative working directory.
	Project *workspace.Project
	// Target is the target of this update. This includes the name of the stack being updated, the configuration values
	// associated with the target and the target's latest snapshot.
	Target *deploy.Target
}

// Context provides cancellation, termination, and eventing options for an engine operation. It also provides
// a way for the engine to persist snapshots, using the `SnapshotManager`.
type Context struct {
	Cancel          *cancel.Context
	Events          chan<- Event
	SnapshotManager SnapshotManager
	BackendClient   deploy.BackendClient
	ParentSpan      opentracing.SpanContext
}
