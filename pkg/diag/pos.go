// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

// Pos represents a position in a file.
type Pos struct {
	Row int
	Col int
}

// EmptyPos may be used when no position is needed.
var EmptyPos = Pos{0, 0}

// Location represents a region spanning two positions in a file.
type Location struct {
	Start Pos
	End   Pos
}

// EmptyLocation may be used when no position information is available.
var EmptyLocation = Location{EmptyPos, EmptyPos}
