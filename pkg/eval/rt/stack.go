// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rt

import (
	"bytes"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
)

// StackFrame is a structure that helps us build up a stack trace upon failure.
type StackFrame struct {
	Parent *StackFrame      // the parent frame.
	Node   diag.Diagable    // the current frame's location.
	Func   symbols.Function // the current function.
	Caller diag.Diagable    // the location inside of our caller.
}

// Trace creates a stack trace from the given stack.  If a current location is given, that will be used for the location
// of the first frame; if it is missing, no location will be given.
func (s *StackFrame) Trace(d diag.Sink, prefix string, current diag.Diagable) string {
	var trace bytes.Buffer

	for s != nil {
		// First print the prefix (tab, spaces, whatever).
		trace.WriteString(prefix)

		// Now produce a string indicating the name and signature of the function; this will look like this:
		//     at package:module:function(A, .., Z)R
		// where A are the argument types (if any) and R is the return type (if any).
		trace.WriteString("at ")
		if s.Func == nil {
			trace.WriteString("lambda")
			if s.Node != nil {
				if doc, loc := s.Node.Where(); doc != nil || loc != nil {
					trace.WriteString(" (")
					trace.WriteString(d.StringifyLocation(doc, loc))
					trace.WriteString(")")
				}
			}
		} else {
			trace.WriteString(string(s.Func.Token()))
			trace.WriteString(string(s.Func.Signature().Token()))
		}

		// Next, if there's source information about the current location inside of this function, print it.
		if current != nil {
			if doc, loc := current.Where(); doc != nil || loc != nil {
				trace.WriteString(" in ")
				trace.WriteString(d.StringifyLocation(doc, loc))
			}
		}

		// Remember the current frame's caller position as our next frame's current position.
		current = s.Caller

		// Now advance to the parent (or break out if we have reached the top).
		s = s.Parent
		if s != nil {
			trace.WriteString("\n")
		}
	}

	return trace.String()
}
