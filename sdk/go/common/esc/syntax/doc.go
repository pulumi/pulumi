// Copyright 2022, Pulumi Corporation.  All rights reserved.

package syntax

// The syntax package defines a syntax tree for a JSON-like object language. The language
// supports null, boolean, number, and string literals, arrays, and objects. Each node in
// the tree may carry low-level syntactical information associated with the input syntax.
// This low-level information includes positional information, and in the future may contain
// syntactical trvia such as comments.
