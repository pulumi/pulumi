// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
	return URN(
		URNPrefix +
			string(ns) +
			URNNameDelimiter + string(alloc) +
			URNNameDelimiter + string(t) +
			URNNameDelimiter + string(name),
	)
}

// URNName returns the URN name part of a URN (i.e., strips off the prefix).
func (urn URN) URNName() string {
	s := string(urn)
	contract.Assert(strings.HasPrefix(s, URNPrefix))
	return s[len(URNPrefix):]
}

// Namespace returns the resource namespace part of a URN.
func (urn URN) Namespace() tokens.QName {
	return tokens.QName(strings.Split(urn.URNName(), URNNameDelimiter)[0])
}

// Alloc returns the resource allocation context part of a URN.
func (urn URN) Alloc() tokens.Module {
	return tokens.Module(strings.Split(urn.URNName(), URNNameDelimiter)[1])
}

// Type returns the resource type part of a URN.
func (urn URN) Type() tokens.Type {
	return tokens.Type(strings.Split(urn.URNName(), URNNameDelimiter)[2])
}

// Name returns the resource name part of a URN.
func (urn URN) Name() tokens.QName {
	return tokens.QName(strings.Split(urn.URNName(), URNNameDelimiter)[3])
}
