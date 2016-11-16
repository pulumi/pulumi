// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"strings"
)

// NameDelimiter is what delimits Namespace and Name parts.
const NameDelimiter = "/"

// NamePart extracts the name portion of a Name (dropping any Namespace).
func NamePart(nm Name) Name {
	ix := strings.LastIndex(string(nm), NameDelimiter)
	if ix == -1 {
		return nm
	}
	return nm[ix+1:]
}

// NamespacePart extracts the namespace portion of a Name (dropping the Name); this may be empty.
func NamespacePart(nm Name) Name {
	ix := strings.LastIndex(string(nm), NameDelimiter)
	if ix == -1 {
		return ""
	}
	return nm[:ix]
}
