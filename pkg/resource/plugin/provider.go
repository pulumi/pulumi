// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"io"

	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
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
	// Check validates that the given property bag is valid for a resource of the given type.
	Check(t tokens.Type, props resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error)
	// Diff checks what impacts a hypothetical update will have on the resource's properties.
	Diff(t tokens.Type, id resource.ID, olds resource.PropertyMap, news resource.PropertyMap) (DiffResult, error)
	// Create allocates a new instance of the provided resource and returns its unique resource.ID.
	Create(t tokens.Type, props resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error)
	// Get reads the instance state identified by res and returns it.
	Get(t tokens.Type, id resource.ID) (resource.PropertyMap, error)
	// Update updates an existing resource with new values.
	Update(t tokens.Type, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error)
	// Delete tears down an existing resource.
	Delete(t tokens.Type, id resource.ID, props resource.PropertyMap) (resource.Status, error)
}

// CheckFailure indicates that a call to check failed; it contains the property and reason for the failure.
type CheckFailure struct {
	Property resource.PropertyKey // the property that failed checking.
	Reason   string               // the reason the property failed to check.
}

// DiffResult indicates whether an operation should replace or update an existing resource.
type DiffResult struct {
	ReplaceKeys []resource.PropertyKey // an optional list of replacement keys.
}

// Replace returns true if this diff represents a replacement.
func (r DiffResult) Replace() bool {
	return len(r.ReplaceKeys) > 0
}
