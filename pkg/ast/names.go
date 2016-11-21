// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"strings"
)

// NameDelimiter is what delimits Namespace and Name parts.
const NameDelimiter = "/"

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
	pix := strings.Index(s, "://")
	if pix != -1 {
		// Remember it and then strip it off for subsequent parsing.
		parts.Proto = s[:pix]
		s = s[pix+1:]
	}

	// Now look to see if there is a dot, indicating a base part.
	bix := strings.Index(s, ".")
	if bix == -1 {
		// No base seems to be here; populate it with the default ref base.
		// TODO(joe): this might be questionable; e.g., domain-less hosts will require a trailing period.
		parts.Base = DefaultRefBase
	} else {
		// A base exists; look for a slash (indicating the name), and capture everything up to it.
		six := strings.Index(s[bix+1:], NameDelimiter)
		if six == -1 {
			parts.Base = s
			s = ""
		} else {
			rest := bix + 1 + six
			parts.Base = s[:rest]
			s = s[rest+1:]
		}
	}

	// Anything remaining at this point represents the name.
	parts.Name = Name(s)

	return parts
}
