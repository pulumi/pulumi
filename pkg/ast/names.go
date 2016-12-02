// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"regexp"
	"strings"

	"github.com/marapongo/mu/pkg/util"
)

// NameDelimiter is what delimits Namespace and Name parts.
const NameDelimiter = "/"

var NameRegexp = regexp.MustCompile(NameRegexps)
var NameRegexps = "(" + NamePartRegexps + "\\" + NameDelimiter + ")*" + NamePartRegexps
var NamePartRegexps = "[A-Za-z_][A-Za-z0-9_]*"

// IsName checks whether a string is a legal Name.
func IsName(s string) bool {
	return NameRegexp.FindString(s) == s
}

// AsName converts a given string to a Name, asserting its validity.
func AsName(s string) Name {
	util.AssertMF(NameRegexp.MatchString(s), "Expected string '%v' to be a name (%v)", s, NameRegexps)
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
