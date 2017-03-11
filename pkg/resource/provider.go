// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"io"

	"github.com/pulumi/coconut/pkg/tokens"
)

// Provider presents a simple interface for orchestrating resource create, reead, update, and delete operations.  Each
// provider understands how to handle all of the resource types within a single package.
//
// This interface hides some of the messiness of the underlying machinery, since providers are behind an RPC boundary.
//
// It is important to note that provider operations are not transactional.  (Some providers might decide to offer
// transactional semantics, but such a provider is a rare treat.)  As a result, failures in the operations below can
// range from benign to catastrophic (possibly leaving behind a corrupt resource).  It is up to the provider to make a
// best effort to ensure catastrophies do not occur.  The errors returned from mutating operations indicate both the
// underlying error condition in addition to a bit indicating whether the operation was successfully rolled back.
type Provider interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer
	// Pkg fetches this provider's package.
	Pkg() tokens.Package
	// Check validates that the given property bag is valid for a resource of the given type.
	Check(t tokens.Type, props PropertyMap) ([]CheckFailure, error)
	// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
	// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
	// In any case, resources with the same name must be safe to use interchangeably with one another.
	Name(t tokens.Type, props PropertyMap) (tokens.QName, error)
	// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
	Create(t tokens.Type, props PropertyMap) (ID, error, ResourceState)
	// Read reads the instance state identified by id/t, and returns a bag of properties.
	Read(id ID, t tokens.Type) (PropertyMap, error)
	// Update updates an existing resource with new values.
	Update(id ID, t tokens.Type, olds PropertyMap, news PropertyMap) (error, ResourceState)
	// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
	UpdateImpact(id ID, t tokens.Type, olds PropertyMap, news PropertyMap) ([]string, PropertyMap, error)
	// Delete tears down an existing resource.
	Delete(id ID, t tokens.Type) (error, ResourceState)
}

// CheckFailure indicates that a call to check failed; it contains the property and reason for the failure.
type CheckFailure struct {
	Property PropertyKey // the property that failed checking.
	Reason   string      // the reason the property failed to check.
}
