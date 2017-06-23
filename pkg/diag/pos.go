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

package diag

// Pos represents a position in a file.
type Pos struct {
	Line   int // a 1-based line number
	Column int // a 1-based column number
}

// EmptyPos may be used when no position is needed.
var EmptyPos = Pos{0, 0}

// IsEmpty returns true if the Pos information is missing.
func (pos Pos) IsEmpty() bool {
	return pos.Line == 0 && pos.Column == 0
}

// Location represents a region spanning two positions in a file.
type Location struct {
	Start Pos  // a starting position.
	End   *Pos // an ending position; if nil, represents a point.
}

// EmptyLocation may be used when no position information is available.
var EmptyLocation = Location{EmptyPos, nil}

// IsEmpty returns true if the Location information is missing.
func (loc Location) IsEmpty() bool {
	return loc.Start.IsEmpty() && (loc.End == nil || loc.End.IsEmpty())
}
