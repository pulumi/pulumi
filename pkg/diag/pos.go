// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

// Pos represents a position in a file.
type Pos struct {
	Row int
	Col int
}

// EmptyPos may be used when no position is needed.
var EmptyPos = Pos{0, 0}

// PosRange represents a position range in a file.
type PosRange struct {
	Start Pos
	End   Pos
}

// EmptyPosRange may be used when no position range is needed.
var EmptyPosRange = PosRange{EmptyPos, EmptyPos}
