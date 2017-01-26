// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"regexp"
	"strings"

	"github.com/marapongo/mu/pkg/util/contract"
)

// Name is an identifier.  It conforms to the regex [A-Za-z_.][A-Za-z0-9_]*.
type Name string

var NameRegexp = regexp.MustCompile(NameRegexpPattern)
var NameRegexpPattern = "[A-Za-z_.][A-Za-z0-9_]*"

// Q turns a Name into a qualified name; this is legal, since Name's is a proper subset of QName's grammar.
func (nm Name) Q() QName { return QName(nm) }

// IsName checks whether a string is a legal Name.
func IsName(s string) bool {
	return s != "" && NameRegexp.FindString(s) == s
}

// AsName converts a given string to a Name, asserting its validity.
func AsName(s string) Name {
	contract.Assertf(IsName(s), "Expected string '%v' to be a name (%v)", s, NameRegexpPattern)
	return Name(s)
}

// QName is a qualified identifier.  The "/" character optionally delimits different pieces of the name.  Each element
// conforms to the Name regex [A-Za-z_][A-Za-z0-9_]*.  For example, "marapongo/mu/stack".
type QName string

// QNameDelimiter is what delimits Namespace and Name parts.
const QNameDelimiter = "/"

var QNameRegexp = regexp.MustCompile(QNameRegexpPattern)
var QNameRegexpPattern = "(" + NameRegexpPattern + "\\" + QNameDelimiter + ")*" + NameRegexpPattern

// IsQName checks whether a string is a legal Name.
func IsQName(s string) bool {
	return s != "" && QNameRegexp.FindString(s) == s
}

// AsQName converts a given string to a QName, asserting its validity.
func AsQName(s string) QName {
	contract.Assertf(IsQName(s), "Expected string '%v' to be a name (%v)", s, QNameRegexpPattern)
	return QName(s)
}

// Name extracts the Name portion of a QName (dropping any namespace).
func (nm QName) Name() Name {
	ix := strings.LastIndex(string(nm), QNameDelimiter)
	var nmn string
	if ix == -1 {
		nmn = string(nm)
	} else {
		nmn = string(nm[ix+1:])
	}
	contract.Assert(IsName(nmn))
	return Name(nmn)
}

// Namespace extracts the namespace portion of a QName (dropping the name); this may be empty.
func (nm QName) Namespace() QName {
	ix := strings.LastIndex(string(nm), QNameDelimiter)
	var qn string
	if ix == -1 {
		qn = ""
	} else {
		qn = string(nm[:ix])
	}
	contract.Assert(IsQName(qn))
	return QName(qn)
}
