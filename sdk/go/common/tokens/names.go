// Copyright 2016-2018, Pulumi Corporation.
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

package tokens

import (
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Name is an identifier.  It conforms to the regex [A-Za-z_.][A-Za-z0-9_]*.
type Name string

func (nm Name) String() string { return string(nm) }

// Q turns a Name into a qualified name; this is legal, since Name's is a proper subset of QName's grammar.
func (nm Name) Q() QName { return QName(nm) }

var NameRegexp = regexp.MustCompile(NameRegexpPattern)
var NameRegexpPattern = "[A-Za-z_.][A-Za-z0-9_.]*"

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
// conforms to the Name regex [A-Za-z_.][A-Za-z0-9_.]*.  For example, "pulumi/pulumi/stack".
type QName string

func (nm QName) String() string { return string(nm) }

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

// PackageName is a qualified name referring to an imported package.  It is similar to a QName, except that it permits
// dashes "-" as is commonplace with packages of various kinds.
type PackageName string

func (nm PackageName) String() string { return string(nm) }

var PackageNameRegexp = regexp.MustCompile(PackageNameRegexpPattern)
var PackagePartRegexpPattern = "[A-Za-z_.][A-Za-z0-9_.-]*"
var PackageNameRegexpPattern = "(" + PackagePartRegexpPattern + "\\" + QNameDelimiter + ")*" + PackagePartRegexpPattern

// IsPackageName checks whether a string is a legal Name.
func IsPackageName(s string) bool {
	return s != "" && PackageNameRegexp.FindString(s) == s
}

// ModuleName is a qualified name referring to an imported module from a package.
type ModuleName QName

func (nm ModuleName) String() string { return string(nm) }

// ModuleMemberName is a simple name representing the module member's identifier.
type ModuleMemberName Name

func (nm ModuleMemberName) String() string { return string(nm) }

// ClassMemberName is a simple name representing the class member's identifier.
type ClassMemberName Name

func (nm ClassMemberName) Name() Name     { return Name(nm) }
func (nm ClassMemberName) String() string { return string(nm) }

// TypeName is a simple name representing the type's name, without any package/module qualifiers.
type TypeName Name

func (nm TypeName) String() string { return string(nm) }
