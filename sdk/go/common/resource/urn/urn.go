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

package urn

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
type URN string

const (
	Prefix        = "urn:" + NamespaceID + ":" // the standard URN prefix
	NamespaceID   = "pulumi"                   // the URN namespace
	NameDelimiter = "::"                       // the delimiter between URN name elements
	TypeDelimiter = "$"                        // the delimiter between URN type elements
)

// Parse attempts to parse a string into a URN returning an error if it's not valid.
func Parse(s string) (URN, error) {
	if s == "" {
		return "", errors.New("missing required URN")
	}

	urn := URN(s)
	if !urn.IsValid() {
		return "", fmt.Errorf("invalid URN %q", s)
	}
	return urn, nil
}

// ParseOptional is the same as Parse except it will allow the empty string.
func ParseOptional(s string) (URN, error) {
	if s == "" {
		return "", nil
	}
	return Parse(s)
}

// New creates a unique resource URN for the given resource object.
func New(stack tokens.QName, proj tokens.PackageName, parentType, baseType tokens.Type, name string) URN {
	typ := string(baseType)
	if parentType != "" && parentType != tokens.RootStackType {
		typ = string(parentType) + TypeDelimiter + typ
	}

	return URN(
		Prefix +
			string(stack) +
			NameDelimiter + string(proj) +
			NameDelimiter + typ +
			NameDelimiter + name,
	)
}

// Quote returns the quoted form of the URN appropriate for use as a command line argument for the current OS.
func (urn URN) Quote() string {
	quote := `'`
	if runtime.GOOS == "windows" {
		// Windows uses double-quotes instead of single-quotes.
		quote = `"`
	}
	return quote + string(urn) + quote
}

// IsValid returns true if the URN is well-formed.
func (urn URN) IsValid() bool {
	if !strings.HasPrefix(string(urn), Prefix) {
		return false
	}

	return strings.Count(string(urn), NameDelimiter) >= 3
	// TODO: We should validate the stack, project and type tokens here, but currently those fields might not
	// actually be "valid" (e.g. spaces in project names, custom component types, etc).
}

// URNName returns the URN name part of a URN (i.e., strips off the prefix).
func (urn URN) URNName() string {
	s := string(urn)
	contract.Assertf(strings.HasPrefix(s, Prefix), "Urn is: '%s'", string(urn))
	return s[len(Prefix):]
}

// Stack returns the resource stack part of a URN.
func (urn URN) Stack() tokens.QName {
	return tokens.QName(getComponent(urn.URNName(), NameDelimiter, 0))
}

// Project returns the project name part of a URN.
func (urn URN) Project() tokens.PackageName {
	return tokens.PackageName(getComponent(urn.URNName(), NameDelimiter, 1))
}

// QualifiedType returns the resource type part of a URN including the parent type
func (urn URN) QualifiedType() tokens.Type {
	return tokens.Type(getComponent(urn.URNName(), NameDelimiter, 2))
}

// Gets the n'th delimited component of a string.
//
// This is used instead of the `strings.Split(string, delimiter)[index]` pattern which
// is inefficient.
func getComponent(input string, delimiter string, index int) string {
	return getComponentN(input, delimiter, index, false)
}

// This gets the n'th delimited compnent of a string, and optionally the rest of the string
//
// If the *open* parameter is true, then this will return everything after the n-1th delimiter
func getComponentN(input string, delimiter string, index int, open bool) string {
	if open && index == 0 {
		return input
	}
	nameDelimiters := 0
	partStart := 0
	for i := 0; i < len(input); i++ {
		if strings.HasPrefix(input[i:], delimiter) {
			nameDelimiters++
			if nameDelimiters == index {
				i += len(delimiter)
				partStart = i
				if open {
					return input[partStart:]
				}
				i--
			} else if nameDelimiters > index {
				return input[partStart:i]
			} else {
				i += len(delimiter) - 1
			}
		}
	}
	return input[partStart:]
}

// Type returns the resource type part of a URN
func (urn URN) Type() tokens.Type {
	name := urn.URNName()
	qualifiedType := getComponent(name, NameDelimiter, 2)

	lastTypeDelimiter := strings.LastIndex(qualifiedType, TypeDelimiter)
	return tokens.Type(qualifiedType[lastTypeDelimiter+1:])
}

// Name returns the resource name part of a URN.
func (urn URN) Name() string {
	return getComponentN(urn.URNName(), NameDelimiter, 3, true)
}

// Returns a new URN with an updated name part
func (urn URN) Rename(newName string) URN {
	return New(
		urn.Stack(),
		urn.Project(),
		// parent type is empty because the qualified type already includes it
		"",
		urn.QualifiedType(),
		newName,
	)
}

// Returns a new URN with an updated stack part
func (urn URN) RenameStack(stack tokens.StackName) URN {
	return New(
		stack.Q(),
		urn.Project(),
		// parent type is empty because the qualified type already includes it
		"",
		urn.QualifiedType(),
		urn.Name(),
	)
}

// Returns a new URN with an updated project part
func (urn URN) RenameProject(project tokens.PackageName) URN {
	return New(
		urn.Stack(),
		project,
		// parent type is empty because the qualified type already includes it
		"",
		urn.QualifiedType(),
		urn.Name(),
	)
}
