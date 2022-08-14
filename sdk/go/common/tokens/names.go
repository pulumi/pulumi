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

// Name is an identifier.  It conforms to NameRegexpPattern.
type Name string

func (nm Name) String() string { return string(nm) }

// Q turns a Name into a qualified name; this is legal, since Name's is a proper subset of QName's grammar.
func (nm Name) Q() QName { return QName(nm) }

var NameRegexp = regexp.MustCompile(NameRegexpPattern)
var nameFirstCharRegexp = regexp.MustCompile("^" + nameFirstCharRegexpPattern + "$")
var nameRestCharRegexp = regexp.MustCompile("^" + nameRestCharRegexpPattern + "$")

var NameRegexpPattern = nameFirstCharRegexpPattern + nameRestCharRegexpPattern

const nameFirstCharRegexpPattern = "[A-Za-z0-9_.-]"
const nameRestCharRegexpPattern = "[A-Za-z0-9_.-]*"

// IsName checks whether a string is a legal Name.
func IsName(s string) bool {
	return s != "" && NameRegexp.FindString(s) == s
}

// QName is a qualified identifier.  The "/" character optionally delimits different pieces of the name.  Each element
// conforms to NameRegexpPattern.  For example, "pulumi/pulumi/stack".
type QName string

func (nm QName) String() string { return string(nm) }

// QNameDelimiter is what delimits Namespace and Name parts.
const QNameDelimiter = "/"

var QNameRegexp = regexp.MustCompile(QNameRegexpPattern)
var QNameRegexpPattern = "(" + NameRegexpPattern + "\\" + QNameDelimiter + ")*" + NameRegexpPattern

// IsQName checks whether a string is a legal QName.
func IsQName(s string) bool {
	return s != "" && QNameRegexp.FindString(s) == s
}

// IntoQName converts an arbitrary string into a QName, converting the string to a valid QName if
// necessary. The conversion is deterministic, but also lossy.
func IntoQName(s string) QName {
	output := []string{}
	for _, s := range strings.Split(s, QNameDelimiter) {
		if s == "" {
			continue
		}
		segment := []byte(s)
		if !nameFirstCharRegexp.Match([]byte{segment[0]}) {
			segment[0] = '_'
		}
		for i := 1; i < len(s); i++ {
			if !nameRestCharRegexp.Match([]byte{segment[i]}) {
				segment[i] = '_'
			}
		}
		output = append(output, string(segment))
	}
	result := strings.Join(output, QNameDelimiter)
	if result == "" {
		result = "_"
	}
	return QName(result)
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

// PackageName is a qualified name referring to an imported package.
type PackageName QName

func (nm PackageName) String() string { return string(nm) }

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
