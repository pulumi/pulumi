// Copyright 2017-2018, Pulumi Corporation.
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
	"io"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Provider presents a simple interface for orchestrating resource create, reead, update, and delete operations.  Each
// provider understands how to handle all of the resource types within a single package.
//
// This interface hides some of the messiness of the underlying machinery, since providers are behind an RPC boundary.
//
// It is important to note that provider operations are not transactional.  (Some providers might decide to offer
// transactional semantics, but such a provider is a rare treat.)  As a result, failures in the operations below can
// range from benign to catastrophic (possibly leaving behind a corrupt resource).  It is up to the provider to make a
// best effort to ensure catastrophes do not occur.  The errors returned from mutating operations indicate both the
// underlying error condition in addition to a bit indicating whether the operation was successfully rolled back.
type Provider interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer
	// Pkg fetches this provider's package.
	Pkg() tokens.Package
	// Configure configures the resource provider with "globals" that control its behavior.
	Configure(vars map[config.Key]string) error
	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource.
	Check(urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool) (resource.PropertyMap, []CheckFailure, error)
	// Diff checks what impacts a hypothetical update will have on the resource's properties.
	Diff(urn resource.URN, id resource.ID, olds resource.PropertyMap, news resource.PropertyMap,
		allowUnknowns bool) (DiffResult, error)
	// Create allocates a new instance of the provided resource and returns its unique resource.ID.
	Create(urn resource.URN, news resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error)
	// Update updates an existing resource with new values.
	Update(urn resource.URN, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error)
	// Delete tears down an existing resource.
	Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error)
	// Invoke dynamically executes a built-in function in the provider.
	Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (workspace.PluginInfo, error)
}

// CheckFailure indicates that a call to check failed; it contains the property and reason for the failure.
type CheckFailure struct {
	Property resource.PropertyKey // the property that failed checking.
	Reason   string               // the reason the property failed to check.
}

// DiffResult indicates whether an operation should replace or update an existing resource.
type DiffResult struct {
	ReplaceKeys         []resource.PropertyKey // an optional list of replacement keys.
	StableKeys          []resource.PropertyKey // an optional list of property keys that are stable.
	DeleteBeforeReplace bool                   // if true, this resource must be deleted before recreating it.
}

// Replace returns true if this diff represents a replacement.
func (r DiffResult) Replace() bool {
	return len(r.ReplaceKeys) > 0
}
