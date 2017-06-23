// Copyright 2016-2017, Pulumi Corporation
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

package ast

// Location is a location, possibly a region, in the source code.
type Location struct {
	File  *string   `json:"file,omitempty"` // an optional filename in which this location resides.
	Start Position  `json:"start"`          // a starting position.
	End   *Position `json:"end,omitempty"`  // an optional end position for a range (if nil, just a point).
}

// Position consists of a 1-indexed `line` number and a 0-indexed `column` number.
type Position struct {
	Line   int64 `json:"line"`   // a 1-based line number
	Column int64 `json:"column"` // a 0-based column number
}
