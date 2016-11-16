// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

// Pos represents a position in a file.
type Pos struct {
	Ln  int
	Col int
}

// EmptyPos may be used when no position is needed.
var EmptyPos = Pos{0, 0}

// IsEmpty returns true if the Pos information is missing.
func (pos Pos) IsEmpty() bool {
	return pos.Ln == 0 && pos.Col == 0
}

// Location represents a region spanning two positions in a file.
type Location struct {
	From Pos
	To   Pos
}

// EmptyLocation may be used when no position information is available.
var EmptyLocation = Location{EmptyPos, EmptyPos}

// IsEmpty returns true if the Location information is missing.
func (loc Location) IsEmpty() bool {
	return loc.From.IsEmpty() && loc.To.IsEmpty()
}
