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
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A provider reference is (URN, ID) tuple that refers to a particular provider instance. A provider reference's
// string representation is <URN> "::" <ID>. The URN's type portion must be of the form "pulumi:providers:<pkg>".

// UnknownID is a distinguished token used to indicate that a provider's ID is not known (e.g. because we are
// performing a preview).
const UnknownID = plugin.UnknownStringValue

// UnconfiguredID is a distinguished token used to indicate that a provider doesn't yet have an ID because it hasn't
// been configured yet. This should never be returned back to SDKs by the engine but is used for internal tracking so we
// maximally reuse provider instances but only configure them once.
const UnconfiguredID = "unconfigured"

// IsProviderType returns true if the supplied type token refers to a Pulumi provider.
func IsProviderType(typ tokens.Type) bool {
	// Tokens without a module member are definitely not provider types.
	if !tokens.Token(typ).HasModuleMember() {
		return false
	}
	return typ.Module() == "pulumi:providers" && typ.Name() != ""
}

// IsDefaultProvider returns true if this URN refers to a default Pulumi provider.
func IsDefaultProvider(urn resource.URN) bool {
	return IsProviderType(urn.Type()) && strings.HasPrefix(urn.Name(), "default")
}

// MakeProviderType returns the provider type token for the given package.
func MakeProviderType(pkg tokens.Package) tokens.Type {
	return tokens.Type("pulumi:providers:" + pkg)
}

// GetProviderPackage returns the provider package for the given type token.
func GetProviderPackage(typ tokens.Type) tokens.Package {
	contract.Requiref(IsProviderType(typ), "typ", "must be a provider type token, got %q", typ)
	return tokens.Package(typ.Name())
}

func validateURN(urn resource.URN) error {
	if !urn.IsValid() {
		return fmt.Errorf("%s is not a valid URN", urn)
	}
	typ := urn.Type()
	if typ.Module() != "pulumi:providers" {
		return fmt.Errorf("invalid module in type: expected 'pulumi:providers', got '%v'", typ.Module())
	}
	if typ.Name() == "" {
		return errors.New("provider URNs must specify a type name")
	}
	return nil
}

// Reference represents a reference to a particular provider.
type Reference struct {
	urn resource.URN
	id  resource.ID
}

// URN returns the provider reference's URN.
func (r Reference) URN() resource.URN {
	return r.urn
}

// ID returns the provider reference's ID.
func (r Reference) ID() resource.ID {
	return r.id
}

// String returns the string representation of this provider reference.
func (r Reference) String() string {
	if r.urn == "" && r.id == "" {
		return ""
	}

	return string(r.urn) + resource.URNNameDelimiter + string(r.id)
}

const denyDefaultProviderID resource.ID = "denydefaultprovider"

// DenyDefaultProvider represent a default provider that cannot be created.
func NewDenyDefaultProvider(name string) Reference {
	return mustNewReference(
		resource.NewURN("denied", "denied", "denied", "pulumi:providers:denied", name),
		denyDefaultProviderID)
}

// Retrieves the package of the denied provider.
//
// For example, if a reference to:
// "urn:pulumi:stack::project::pulumi:providers:aws::default_4_35_0"
// was denied, then GetDeniedDefaultProviderPkg would return "aws".
//
// Panics if called on a provider that is not a DenyDefaultProvider.
func GetDeniedDefaultProviderPkg(ref Reference) string {
	contract.Requiref(IsDenyDefaultsProvider(ref), "ref", "must be a DenyDefaultProvider, got %v", ref)
	return ref.URN().Name()
}

func IsDenyDefaultsProvider(ref Reference) bool {
	return ref.id == denyDefaultProviderID
}

// NewReference creates a new reference for the given URN and ID.
func NewReference(urn resource.URN, id resource.ID) (Reference, error) {
	if err := validateURN(urn); err != nil {
		return Reference{}, err
	}
	return Reference{urn: urn, id: id}, nil
}

func mustNewReference(urn resource.URN, id resource.ID) Reference {
	ref, err := NewReference(urn, id)
	contract.AssertNoErrorf(err, "could not create reference with URN '%v' and ID '%v'", urn, id)
	return ref
}

// ParseReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseReference(s string) (Reference, error) {
	// If this is not a valid URN + ID, return false. Note that we don't try terribly hard to validate the URN portion
	// of the reference.
	lastSep := strings.LastIndex(s, resource.URNNameDelimiter)
	if lastSep == -1 {
		return Reference{}, fmt.Errorf("expected '%v' in provider reference '%v'", resource.URNNameDelimiter, s)
	}
	urn, id := resource.URN(s[:lastSep]), resource.ID(s[lastSep+len(resource.URNNameDelimiter):])
	if err := validateURN(urn); err != nil {
		return Reference{}, err
	}
	return Reference{urn: urn, id: id}, nil
}
