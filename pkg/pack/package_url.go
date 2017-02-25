// Copyright 2016 Pulumi, Inc. All rights reserved.

package pack

import (
	"fmt"
	"strings"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// PackageURLString represents a fully qualified "URL-like" reference to an entity, usually another package.  This
// string starts with an optional "protocol" (like https://, git://, etc), followed by an optional "base" part (like
// hub.mu.com/, github.com/, etc), followed by the "name" part (which is just a Name), followed by an optional "#" and
// version number (where version may be "latest", a semantic version range, or a Git SHA hash).
type PackageURLString string

// Parse parses a PackageURLString into a data structure for convenient access to its component parts.
func (u PackageURLString) Parse(pkg tokens.PackageName) (PackageURL, error) {
	s := string(u)
	parsed := PackageURL{}

	// The special "*" URL just takes the package name and uses it as-is, binding to the latest.
	if s == "*" {
		s = string(pkg)
	}

	// Look for the leading protocol, if any.
	protoEnd := strings.Index(s, "://")
	if protoEnd != -1 {
		// Remember it and then strip it off for subsequent parsing.
		parsed.Proto = s[:protoEnd+3]
		s = s[protoEnd+3:]
	}

	// Strip off the version first, so looking for dots doesn't get confused.
	verIndex := strings.Index(s, "#")
	if verIndex != -1 {
		parsed.Version = VersionSpec(s[verIndex+1:])
		if err := parsed.Version.Check(); err != nil {
			return parsed, fmt.Errorf("Illegal version spec in '%v': %v", u, err)
		}
		s = s[:verIndex]
	}

	// Now look to see if there is a dot, indicating a base part.
	dotIndex := strings.Index(s, ".")
	if dotIndex != -1 {
		// A base exists; look for a slash (indicating the name), and capture everything up to it (including it).
		// TODO(joe): this might be questionable; e.g., domain-less hosts will require a trailing period.
		slashIndex := strings.Index(s, tokens.QNameDelimiter)
		if slashIndex == -1 {
			return parsed, fmt.Errorf("Expected a name to follow the base URL in '%v'", u)
		}

		parsed.Base = s[:slashIndex+1]
		s = s[slashIndex+1:]
	}

	// Anything remaining at this point represents the name.
	if s == "" {
		return parsed, fmt.Errorf("Expected a name in '%v'", u)
	}
	if !tokens.IsQName(s) {
		return parsed, fmt.Errorf("Expected a qualified package name in '%v': %v", u, s)
	}

	parsed.Name = tokens.PackageName(s)
	return parsed, nil
}

// MustParse parses the parts of a PackageURLString into a PackageURL, failing fast if parsing fails.
func (u PackageURLString) MustParse(pkg tokens.PackageName) PackageURL {
	p, err := u.Parse(pkg)
	contract.Assertf(err == nil, "Expected a nil error from PackageURLString.Parse(%v); got %v", pkg, err)
	return p
}

// PackageURL represents a parsed PackageURLString.
type PackageURL struct {
	Proto   string             // the protocol (e.g., "https://").
	Base    string             // the base part of the URL (e.g., "cocohub.com/").
	Name    tokens.PackageName // the name part of the URL (e.g., "coconut/container").
	Version VersionSpec        // the version part of the URL (e.g., "#1.0.6").
}

var _ fmt.Stringer = PackageURL{} // compile-time assertion that PackageURL implements Stringer.

// DefaultPackageURLProto is the default URL protocol.
const DefaultPackageURLProto = "https://"

// DefaultPackageURLBase is the base part used if a URL doesn't specify one explicitly.
const DefaultPackageURLBase = "cocohub.com/"

// DefaultPackageURLVersion is the default URL version if none is specified.
const DefaultPackageURLVersion = LatestVersion

// Defaults replaces any empty parts of a PackageURL with their default values.
func (u PackageURL) Defaults() PackageURL {
	d := u
	if d.Proto == "" {
		d.Proto = DefaultPackageURLProto
	}
	if d.Base == "" {
		d.Base = DefaultPackageURLBase
	}
	if string(d.Version) == "" {
		d.Version = DefaultPackageURLVersion
	}
	return d
}

func (u PackageURL) URL() PackageURLString {
	return PackageURLString(u.String())
}

func (u PackageURL) String() string {
	s := u.Proto + u.Base + string(u.Name)
	if string(u.Version) != "" {
		s += "#" + string(u.Version)
	}
	return s
}
