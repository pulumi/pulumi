// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

// Location is a location, possibly a region, in the source code.
type Location struct {
	File  *string   `json:"file,omitempty"` // an optional filename in which this location resides.
	Start Position  `json:"start"`          // a starting position.
	End   *Position `json:"end,omitempty"`  // an optional end position for a range (if nil, just a point).
}

// Position consists of a 1-indexed `line` number and a 0-indexed `column` number.
type Position struct {
	Line   int64 `json:"line"`   // >= 1
	Column int64 `json:"column"` // >= 0
}
