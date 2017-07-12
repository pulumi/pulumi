// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
