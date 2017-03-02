// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"strings"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Moniker is a friendly, but unique, name for a resource, most often auto-assigned by Coconut.  These monikers
// are used as unique IDs for objects, and help to to perform graph diffing and resolution of resource object changes.
//
// In theory, we could support manually assigned monikers in the future (e.g., think UUIDs).  For the time being,
// however, we have opted to simplify developers' lives by mostly automating the process.  The one caveat where it isn't
// truly automatic is that a developer -- or resource provider -- must provide a semi-unique name.
//
// Each moniker is of the form:
//
//     <Namespace>::<AllocModule>::<Type>::<Name>
//
// wherein each element is the following:
//
//     <Namespace>      The namespace being deployed into
//     <AllocModule>    The module token in which the object was allocated
//     <Type>           The object type's full type token
//     <Name>           The human-friendly name identifier assigned by the developer or provider
//
// In the future, we may add elements to the moniker; it is more important that it is unique than it is human-typable.
type Moniker string

const MonikerDelimiter = "::" // the delimiter between elements of the moniker.

// NewMoniker creates a unique moniker for the given object.
func NewMoniker(ns tokens.QName, alloc tokens.Module, t tokens.Type, name tokens.QName) Moniker {
	m := Moniker(
		string(ns) +
			MonikerDelimiter + string(alloc) +
			MonikerDelimiter + string(t) +
			MonikerDelimiter + string(name),
	)
	contract.Assert(!m.Replacement())
	return m
}

// replaceMonikerSuffix is the suffix for monikers referring to resources that are being replaced.
const replaceMonikerSuffix = Moniker("#<new-id(replace)>")

// Replace returns a new, modified replacement moniker (used to tag resources that are meant to be replaced).
func (m Moniker) Replace() Moniker {
	contract.Assert(!m.Replacement())
	return m + replaceMonikerSuffix
}

// Unreplace returns the underlying replacement's moniker.
func (m Moniker) Unreplace() Moniker {
	contract.Assert(m.Replacement())
	return m[:len(m)-len(replaceMonikerSuffix)]
}

// Replacement returns true if this moniker refers to a resource that is meant to be replaced.
func (m Moniker) Replacement() bool {
	return strings.HasSuffix(string(m), string(replaceMonikerSuffix))
}
