// Copyright 2016 Marapongo, Inc. All rights reserved.

// Location is a location, possibly a region, in the source code.
export interface Location {
    file?: string;   // an optional filename in which this location resides.
    start: Position; // a starting position.
    end?:  Position; // an optional end position for a range (if empty, this represents just a point).
}

// Position consists of a 1-indexed `line` number and a 0-indexed `column` number.
export interface Position {
    line:   number; // >= 1
    column: number; // >= 0
}

