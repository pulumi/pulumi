// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"errors"
	"regexp"

	"github.com/blang/semver"
)

// Version represents a precise version number.  It may be either a Git SHA hash or a semantic version (not a range).
type Version string

// VersionSpec represents a specification of a version that is bound to a precise number through a separate process.
// It may take the form of a Version (see above), a semantic version range, or the string "latest", to indicate that the
// latest available sources are to be used at compile-time.
type VersionSpec string

var sha1HashRegexps = "[0-9a-fA-F]"
var shortSHA1HashRegexp = regexp.MustCompile(sha1HashRegexps + "{7}")
var longSHA1HashRegexp = regexp.MustCompile(sha1HashRegexps + "{40}")

// Check ensures the given Version is valid; if it is, Check returns nil; if not, an error describing why is returned.
func (v Version) Check() error {
	// The only three legal values for Version are: a semantic version or a Git SHA hash (short or long).
	// Not that ranges are explicitly disallowed with Versions; for those, you'd use a VersionSpec.
	vs := string(v)
	if vs == "" {
		return errors.New("Missing version")
	}
	if shortSHA1HashRegexp.FindString(vs) == vs {
		return nil
	}
	if longSHA1HashRegexp.FindString(vs) == vs {
		return nil
	}
	_, err := semver.Parse(vs)
	return err
}

// LatestVersion indicates that the latest known source version should be used.
const LatestVersion VersionSpec = "latest"

// Check ensures the given Version is valid; if it is, Check returns nil; if not, an error describing why is returned.
func (v VersionSpec) Check() error {
	// More legal values are permitted here.  Any valid Version is also a valid VersionSpec.  However, VersionSpecs
	// permit the special LatestVersionSpec string, in addition to semantic version ranges.
	if v == LatestVersion {
		return nil
	}
	if err := Version(v).Check(); err == nil {
		return nil
	}

	vs := string(v)
	if vs == "" {
		return errors.New("Missing version")
	}
	_, err := semver.ParseRange(vs)
	// TODO[marapongo/mu#18]: consider supporting the sugared NPM-style semvers, like tilde and caret ranges.
	return err
}
