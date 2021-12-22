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

package resource

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A provider reference is (URN, ID) tuple that refers to a particular provider instance. A provider reference's
// string representation is <URN> "::" <ID>. The URN's type portion must be of the form "pulumi:providers:<pkg>".

// UnknownID is a distinguished token used to indicate that a provider's ID is not known (e.g. because we are
// performing a preview).
const UnknownID = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

// IsProviderType returns true if the supplied type token refers to a Pulumi provider.
func IsProviderType(typ tokens.Type) bool {
	// Tokens without a module member are definitely not provider types.
	if !tokens.Token(typ).HasModuleMember() {
		return false
	}
	return typ.Module() == "pulumi:providers" && typ.Name() != ""
}

// IsDefaultProvider returns true if this URN refers to a default Pulumi provider.
func IsDefaultProvider(urn URN) bool {
	return IsProviderType(urn.Type()) && strings.HasPrefix(urn.Name().String(), "default")
}

// MakeProviderType returns the provider type token for the given package.
func MakeProviderType(pkg tokens.Package) tokens.Type {
	return tokens.Type("pulumi:providers:" + pkg)
}

// GetProviderPackage returns the provider package for the given type token.
func GetProviderPackage(typ tokens.Type) tokens.Package {
	contract.Require(IsProviderType(typ), "typ")
	return tokens.Package(typ.Name())
}

func validateURN(urn URN) error {
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

// ProviderReference represents a reference to a particular provider.
type ProviderReference struct {
	urn URN
	id  ID
}

// URN returns the provider reference's URN.
func (r ProviderReference) URN() URN {
	return r.urn
}

// ID returns the provider reference's ID.
func (r ProviderReference) ID() ID {
	return r.id
}

// String returns the string representation of this provider reference.
func (r ProviderReference) String() string {
	if r.urn == "" && r.id == "" {
		return ""
	}

	return string(r.urn) + URNNameDelimiter + string(r.id)
}

// NewProviderReference creates a new reference for the given URN and ID.
func NewProviderReference(urn URN, id ID) (ProviderReference, error) {
	if err := validateURN(urn); err != nil {
		return ProviderReference{}, err
	}
	return ProviderReference{urn: urn, id: id}, nil
}

// ParseProviderReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseProviderReference(s string) (ProviderReference, error) {
	// If this is not a valid URN + ID, return false. Note that we don't try terribly hard to validate the URN portion
	// of the reference.
	lastSep := strings.LastIndex(s, URNNameDelimiter)
	if lastSep == -1 {
		return ProviderReference{}, fmt.Errorf("expected '%v' in provider reference '%v'", URNNameDelimiter, s)
	}
	urn, id := URN(s[:lastSep]), ID(s[lastSep+len(URNNameDelimiter):])
	if err := validateURN(urn); err != nil {
		return ProviderReference{}, err
	}
	return ProviderReference{urn: urn, id: id}, nil
}
