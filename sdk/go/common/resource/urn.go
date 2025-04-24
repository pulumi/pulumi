// Copyright 2016-2023, Pulumi Corporation.
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

// The contents of this file have been moved. The logic behind URN now lives in
// "github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn". This file exists to fulfill
// backwards-compatibility requirements. No new declarations should be added here.

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
//	urn:pulumi:<Stack>::<Project>::<Qualified$Type$Name>::<Name>
//
// wherein each element is the following:
//
//	<Stack>                 The stack being deployed into
//	<Project>               The project being evaluated
//	<Qualified$Type$Name>   The object type's qualified type token (including the parent type)
//	<Name>                  The human-friendly name identifier assigned by the developer or provider
//
// In the future, we may add elements to the URN; it is more important that it is unique than it is
// human-typable.
type URN = urn.URN

const (
	URNPrefix        = urn.Prefix        // the standard URN prefix
	URNNamespaceID   = urn.NamespaceID   // the URN namespace
	URNNameDelimiter = urn.NameDelimiter // the delimiter between URN name elements
	URNTypeDelimiter = urn.TypeDelimiter // the delimiter between URN type elements
)

// ParseURN attempts to parse a string into a URN returning an error if it's not valid.
func ParseURN(s string) (URN, error) { return urn.Parse(s) }

// ParseOptionalURN is the same as ParseURN except it will allow the empty string.
func ParseOptionalURN(s string) (URN, error) { return urn.ParseOptional(s) }

// NewURN creates a unique resource URN for the given resource object.
func NewURN(stack tokens.QName, proj tokens.PackageName, parentType, baseType tokens.Type, name string) URN {
	return urn.New(stack, proj, parentType, baseType, name)
}
