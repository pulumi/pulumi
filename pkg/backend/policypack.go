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

package backend

import (
	"context"

	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/result"
)

// PublishOperation publishes a PolicyPack to the backend.
type PublishOperation struct {
	Root    string
	PlugCtx *plugin.Context
	Scopes  CancellationScopeSource
}

// ApplyOperation publishes a PolicyPack to the backend.
type ApplyOperation struct {
	Version int
	Scopes  CancellationScopeSource
}

// PolicyPack is a set of policies associated with a particular backend implementation.
type PolicyPack interface {
	// Ref returns a reference to this PolicyPack.
	Ref() PolicyPackReference
	// Backend returns the backend this PolicyPack is managed by.
	Backend() Backend
	// Publish the PolicyPack to the service.
	Publish(ctx context.Context, op PublishOperation) result.Result
	// Apply the PolicyPack to an organization.
	Apply(ctx context.Context, op ApplyOperation) error
}
