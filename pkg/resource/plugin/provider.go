// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"io"

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Provider presents a simple interface for orchestrating resource create, read, update, and delete operations.  Each
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
	Check(t tokens.Type, props resource.PropertyMap) ([]CheckFailure, error)
	// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
	// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
	// In any case, resources with the same name must be safe to use interchangeably with one another.
	Name(t tokens.Type, props resource.PropertyMap) (tokens.QName, error)
	// Create allocates a new instance of the provided resource and returns its unique resource.ID.
	Create(t tokens.Type, props resource.PropertyMap) (resource.ID, resource.Status, error)
	// Get reads the instance state identified by res and returns it.
	Get(t tokens.Type, id resource.ID) (resource.PropertyMap, error)
	// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
	InspectChange(t tokens.Type, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap) ([]resource.PropertyKey, resource.PropertyMap, error)
	// Update updates an existing resource with new values.
	Update(t tokens.Type, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap) (resource.Status, error)
	// Delete tears down an existing resource.
	Delete(t tokens.Type, id resource.ID) (resource.Status, error)
	// Query returns an array of resource references of a specified type.
	Query(t tokens.Type) ([]*QueryItem, error)
}

// CheckFailure indicates that a call to check failed; it contains the property and reason for the failure.
type CheckFailure struct {
	Property resource.PropertyKey // the property that failed checking.
	Reason   string               // the reason the property failed to check.
}

type QueryItem struct {
	ID   resource.ID          // The ID of the returned resource
	Item resource.PropertyMap // The property map of the returned resource
}
