// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// URN is a friendly, but unique, URN for a resource, most often auto-assigned by Lumi.  These are
// used as unique IDs for objects, and help us to perform graph diffing and resolution of resource objects.
//
// In theory, we could support manually assigned URIs in the future.  For the time being, however, we have opted to
// simplify developers' lives by mostly automating the generation of them algorithmically.  The one caveat where it
// isn't truly automatic is that a developer -- or resource provider -- must provide a semi-unique name part.
//
// Each resource URN is of the form:
//
//     urn:lumi:<Namespace>::<AllocModule>::<Type>::<Name>
//
// wherein each element is the following:
//
//     <Namespace>      The namespace being deployed into
//     <AllocModule>    The module token in which the object was allocated
//     <Type>           The object type's full type token
//     <Name>           The human-friendly name identifier assigned by the developer or provider
//
// In the future, we may add elements to the URN; it is more important that it is unique than it is human-typable.
type URN string

const (
	URNPrefix        = "urn:" + URNNamespaceID + ":" // the standard URN prefix
	URNNamespaceID   = "lumi"                        // the URN namespace
	URNNameDelimiter = "::"                          // the delimiter between URN name elements
)

// NewURN creates a unique resource URN for the given resource object.
func NewURN(ns tokens.QName, alloc tokens.Module, t tokens.Type, name tokens.QName) URN {
	urn := URN(
		URNPrefix +
			string(ns) +
			URNNameDelimiter + string(alloc) +
			URNNameDelimiter + string(t) +
			URNNameDelimiter + string(name),
	)
	contract.Assert(!urn.Replacement())
	return urn
}

// replaceURNSuffix is the suffix for URNs referring to resources that are being replaced.
const replaceURNSuffix = URN("#<new-id(replace)>")

// Name returns the name part of a URN.
func (urn URN) Name() string {
	urns := string(urn)
	contract.Assert(strings.HasPrefix(urns, URNPrefix))
	return urns[len(URNPrefix):]
}

// Replace returns a new, modified replacement URN (used to tag resources that are meant to be replaced).
func (urn URN) Replace() URN {
	contract.Assert(!urn.Replacement())
	return urn + replaceURNSuffix
}

// Unreplace returns the underlying replacement's URN.
func (urn URN) Unreplace() URN {
	contract.Assert(urn.Replacement())
	return urn[:len(urn)-len(replaceURNSuffix)]
}

// Replacement returns true if this URN refers to a resource that is meant to be replaced.
func (urn URN) Replacement() bool {
	return strings.HasSuffix(string(urn), string(replaceURNSuffix))
}
