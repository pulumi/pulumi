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
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// URN is a friendly, but unique, URN for a resource, most often auto-assigned by Pulumi.  These are
// used as unique IDs for objects, and help us to perform graph diffing and resolution of resource
// objects.
//
// In theory, we could support manually assigned URIs in the future.  For the time being, however,
// we have opted to simplify developers' lives by mostly automating the generation of them
// algorithmically.  The one caveat where it isn't truly automatic is that a developer -- or
// resource provider -- must provide a semi-unique name part.
//
// Each resource URN is of the form:
//
//     urn:pulumi:<Stack>::<Project>::<Qualified$Type$Name>::<Name>
//
// wherein each element is the following:
//
//     <Stack>                 The stack being deployed into
//     <Project>               The project being evaluated
//     <Qualified$Type$Name>   The object type's qualified type token (including the parent type)
//     <Name>                  The human-friendly name identifier assigned by the developer or provider
//
// In the future, we may add elements to the URN; it is more important that it is unique than it is
// human-typable.
type URN string

const (
	URNPrefix        = "urn:" + URNNamespaceID + ":" // the standard URN prefix
	URNNamespaceID   = "pulumi"                      // the URN namespace
	URNNameDelimiter = "::"                          // the delimiter between URN name elements
	URNTypeDelimiter = "$"                           // the delimiter between URN type elements
)

// NewURN creates a unique resource URN for the given resource object.
func NewURN(stack tokens.QName, proj tokens.PackageName, parentType, baseType tokens.Type, name tokens.QName) URN {
	typ := string(baseType)
	if parentType != "" {
		typ = string(parentType) + URNTypeDelimiter + typ
	}

	return URN(
		URNPrefix +
			string(stack) +
			URNNameDelimiter + string(proj) +
			URNNameDelimiter + typ +
			URNNameDelimiter + string(name),
	)
}

// IsValid returns true if the URN is well-formed.
func (urn URN) IsValid() bool {
	if !strings.HasPrefix(string(urn), URNPrefix) {
		return false
	}
	return len(strings.Split(string(urn), URNNameDelimiter)) == 4
}

// URNName returns the URN name part of a URN (i.e., strips off the prefix).
func (urn URN) URNName() string {
	s := string(urn)
	contract.Assertf(strings.HasPrefix(s, URNPrefix), "Urn is: '%s'", string(urn))
	return s[len(URNPrefix):]
}

// Stack returns the resource stack part of a URN.
func (urn URN) Stack() tokens.QName {
	return tokens.QName(strings.Split(urn.URNName(), URNNameDelimiter)[0])
}

// Project returns the project name part of a URN.
func (urn URN) Project() tokens.PackageName {
	return tokens.PackageName(strings.Split(urn.URNName(), URNNameDelimiter)[1])
}

// QualifiedType returns the resource type part of a URN including the parent type
func (urn URN) QualifiedType() tokens.Type {
	return tokens.Type(strings.Split(urn.URNName(), URNNameDelimiter)[2])
}

// Type returns the resource type part of a URN
func (urn URN) Type() tokens.Type {
	qualifiedType := strings.Split(urn.URNName(), URNNameDelimiter)[2]
	types := strings.Split(qualifiedType, URNTypeDelimiter)
	lastType := types[len(types)-1]
	return tokens.Type(lastType)
}

// Name returns the resource name part of a URN.
func (urn URN) Name() tokens.QName {
	return tokens.QName(strings.Split(urn.URNName(), URNNameDelimiter)[3])
}
