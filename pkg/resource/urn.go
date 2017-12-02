// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// URN is a friendly, but unique, URN for a resource, most often auto-assigned by Lumi.  These are
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
//     urn:lumi:<Namespace>::<PkgToken>::<Qualified!!Type!!Name>::<Name>
//
// wherein each element is the following:
//
//     <Namespace>             The namespace being deployed into
//     <AllocPkg>              The package in which the object was allocated
//     <Qualified!!Type!!Name> The object type's qualified type token (including the parent type)
//     <Name>                  The human-friendly name identifier assigned by the developer or provider
//
// In the future, we may add elements to the URN; it is more important that it is unique than it is
// human-typable.
type URN string

const (
	URNPrefix        = "urn:" + URNNamespaceID + ":" // the standard URN prefix
	URNNamespaceID   = "pulumi"                      // the URN namespace
	URNNameDelimiter = "::"                          // the delimiter between URN name elements
	URNTypeDelimiter = "!!"                          // the delimiter between URN type elements
)

// massage ensures that the individual components of the URN do not contains sequences of characters
// that will interfere with later parsing of that URN.  For example, a component should not contain
// :: as that is the delimited we use for separating out all components.
func massage(v string) string {
	delimeters := []string{URNNameDelimiter, URNTypeDelimiter}
	replacements := []string{":_:", "!_!"}

	// First, ensure no name contains our special delimeters.
	for {
		v1 := v
		for i, delim := range delimeters {
			v1 = strings.Replace(v1, delim, replacements[i], -1)
		}
		if v1 == v {
			break
		}

		v = v1
	}

	// Names should not start or end with something that could look like a delimiter either.
	if len(v) > 0 {
		for _, delim := range delimeters {
			if v[0] == delim[0] {
				v = "_" + v
			}

			if v[len(v)-1] == delim[0] {
				v = v + "_"
			}
		}
	}

	return v
}

// NewURN creates a unique resource URN for the given resource object.
func NewURN(ns tokens.QName, alloc tokens.PackageName, parentType, baseType tokens.Type, name tokens.QName) URN {
	// note: we do not need to massage parentType.  It will already have been massaged for us.
	return URN(
		URNPrefix +
			massage(string(ns)) +
			URNNameDelimiter + massage(string(alloc)) +
			URNNameDelimiter +
			(string(parentType) + URNTypeDelimiter + massage(string(baseType))) +
			URNNameDelimiter + massage(string(name)),
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
func (urn URN) Alloc() tokens.PackageName {
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
