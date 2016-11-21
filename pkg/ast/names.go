// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"regexp"
	"strings"

	"github.com/marapongo/mu/pkg/util"
)

// NameDelimiter is what delimits Namespace and Name parts.
const NameDelimiter = "/"

var nameRegexp = regexp.MustCompile(nameRegexps)
var nameRegexps = "(" + namePartRegexps + "\\" + NameDelimiter + ")*" + namePartRegexps
var namePartRegexps = "[A-Za-z_][A-Za-z0-9_]*"

// IsName checks whether a string is a legal Name.
func IsName(s string) bool {
	return nameRegexp.FindString(s) == s
}

// AsName converts a given string to a Name, asserting its validity.
func AsName(s string) Name {
	util.AssertMF(nameRegexp.MatchString(s), "Expected string '%v' to be a name (%v)", s, nameRegexps)
	return Name(s)
}

// Simple extracts the name portion of a Name (dropping any Namespace).
func (nm Name) Simple() Name {
	ix := strings.LastIndex(string(nm), NameDelimiter)
	if ix == -1 {
		return nm
	}
	return nm[ix+1:]
}

// Namespace extracts the namespace portion of a Name (dropping the Name); this may be empty.
func (nm Name) Namespace() Name {
	ix := strings.LastIndex(string(nm), NameDelimiter)
	if ix == -1 {
		return ""
	}
	return nm[:ix]
}

// DefaultRefBase is the base part used if a Ref doesn't specify one explicitly.
const DefaultRefBase = "hub.mu.com/"

// Proto extracts the protocol portion of a Ref.
func (r Ref) Proto() string {
	return r.parse().Proto
}

// Base extracts the base portion of a Ref.
func (r Ref) Base() string {
	return r.parse().Base
}

// Name extracts the name portion of a Ref (including the Namespace).
func (r Ref) Name() Name {
	return r.parse().Name
}

type refParts struct {
	Proto string // the protocol (e.g., "https://").
	Base  string // the base part of the URL (e.g., "mu.hub.com/").
	Name  Name   // the name part of the URL (e.g., "mu/container").
}

func (r Ref) parse() refParts {
	s := string(r)
	parts := refParts{}

	// Look for the leading protocol, if any.
	protoEnd := strings.Index(s, "://")
	if protoEnd != -1 {
		// Remember it and then strip it off for subsequent parsing.
		parts.Proto = s[:protoEnd+3]
		s = s[protoEnd+3:]
	}

	// Now look to see if there is a dot, indicating a base part.
	dotIndex := strings.Index(s, ".")
	if dotIndex == -1 {
		// No base seems to be here; populate it with the default ref base.
		// TODO(joe): this might be questionable; e.g., domain-less hosts will require a trailing period.
		parts.Base = DefaultRefBase
	} else {
		// A base exists; look for a slash (indicating the name), and capture everything up to it (including it).
		slashIndex := strings.Index(s, NameDelimiter)
		if slashIndex == -1 {
			parts.Base = s
			s = ""
		} else {
			parts.Base = s[:slashIndex+1]
			s = s[slashIndex+1:]
		}
	}

	// Anything remaining at this point represents the name.
	parts.Name = Name(s)

	return parts
}
