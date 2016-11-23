// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"errors"
	"fmt"
	"strings"
)

// DefaultRefProto is the default ref protocol.
const DefaultRefProto = "https://"

// DefaultRefBase is the base part used if a Ref doesn't specify one explicitly.
const DefaultRefBase = "hub.mu.com/"

// DefaultRefVersion is the default ref version if none is specified.
const DefaultRefVersion = LatestVersion

// RefParts parses the parts of a Ref into a data structure for convenient access.
func (r Ref) Parse() (RefParts, error) {
	s := string(r)
	parsed := RefParts{}

	// Look for the leading protocol, if any.
	protoEnd := strings.Index(s, "://")
	if protoEnd == -1 {
		parsed.Proto = DefaultRefProto
	} else {
		// Remember it and then strip it off for subsequent parsing.
		parsed.Proto = s[:protoEnd+3]
		s = s[protoEnd+3:]
	}

	// Strip off the version first, so looking for dots doesn't get confused.
	verIndex := strings.Index(s, "@")
	if verIndex == -1 {
		parsed.Version = DefaultRefVersion
	} else {
		parsed.Version = VersionSpec(s[verIndex+1:])
		if err := parsed.Version.Check(); err != nil {
			return parsed, errors.New("Illegal version spec: " + err.Error())
		}
		s = s[:verIndex]
	}

	// Now look to see if there is a dot, indicating a base part.
	dotIndex := strings.Index(s, ".")
	if dotIndex == -1 {
		// No base seems to be here; populate it with the default ref base.
		// TODO(joe): this might be questionable; e.g., domain-less hosts will require a trailing period.
		parsed.Base = DefaultRefBase
	} else {
		// A base exists; look for a slash (indicating the name), and capture everything up to it (including it).
		slashIndex := strings.Index(s, NameDelimiter)
		if slashIndex == -1 {
			return parsed, errors.New("Expected a name to follow the base URL")
		} else {
			parsed.Base = s[:slashIndex+1]
			s = s[slashIndex+1:]
		}
	}

	// Anything remaining at this point represents the name.
	parsed.Name = Name(s)

	return parsed, nil
}

// RefParts represents a parsed Ref structure.
type RefParts struct {
	Proto   string      // the protocol (e.g., "https://").
	Base    string      // the base part of the URL (e.g., "mu.hub.com/").
	Name    Name        // the name part of the URL (e.g., "mu/container").
	Version VersionSpec // the version part of the URL (e.g., "^1.0.6").
}

var _ fmt.Stringer = RefParts{} // compile-time assertion that RefParts implements Stringer.

func (r RefParts) String() string {
	return fmt.Sprintf("%v://%v%v@%v", r.Proto, r.Base, r.Name, r.Version)
}
