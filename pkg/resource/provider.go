// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
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
	Create(res Resource) (ID, error, ResourceState)
	Read(id ID, t tokens.Type, props PropertyMap) (Resource, error)
	Update(old Resource, new Resource) (ID, error, ResourceState)
	Delete(res Resource) (error, ResourceState)
}

type provider struct {
	plugin *Plugin
}

var _ Provider = (*provider)(nil)

func NewProvider(pkg tokens.Package) (Provider, error) {
	contract.Failf("NYI")
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
func (p *provider) Create(res Resource) (ID, error, ResourceState) {
	return "", nil, StateOK // TODO: implement this.
}

// Read reads the instance state identified by id/t, and returns resource object (or nil if not found).
func (p *provider) Read(id ID, t tokens.Type, props PropertyMap) (Resource, error) {
	return nil, nil // TODO: implement this.
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *provider) Update(old Resource, new Resource) (ID, error, ResourceState) {
	return "", nil, StateOK // TODO: implement this.
}

// Delete tears down an existing resource.
func (p *provider) Delete(res Resource) (error, ResourceState) {
	return nil, StateOK // TODO: implement this.
}
