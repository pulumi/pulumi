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

package providers

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// A provider reference is (URN, ID) tuple that refers to a particular provider instance. A provider reference's
// string representation is <URN> "::" <ID>. The URN's type portion must be of the form "pulumi:providers:<pkg>".

// UnknownID is a distinguished token used to indicate that a provider's ID is not known (e.g. because we are
// performing a preview).
const UnknownID = resource.UnknownID

// IsProviderType returns true if the supplied type token refers to a Pulumi provider.
func IsProviderType(typ tokens.Type) bool {
	return resource.IsProviderType(typ)
}

// IsDefaultProvider returns true if this URN refers to a default Pulumi provider.
func IsDefaultProvider(urn resource.URN) bool {
	return resource.IsDefaultProvider(urn)
}

// MakeProviderType returns the provider type token for the given package.
func MakeProviderType(pkg tokens.Package) tokens.Type {
	return resource.MakeProviderType(pkg)
}

// GetProviderPackage returns the provider package for the given type token.
func GetProviderPackage(typ tokens.Type) tokens.Package {
	return resource.GetProviderPackage(typ)
}

// Reference represents a reference to a particular provider.
type Reference = resource.ProviderReference

// NewReference creates a new reference for the given URN and ID.
func NewReference(urn resource.URN, id resource.ID) (Reference, error) {
	return resource.NewProviderReference(urn, id)
}

// ParseReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseReference(s string) (Reference, error) {
	return resource.ParseProviderReference(s)
}
