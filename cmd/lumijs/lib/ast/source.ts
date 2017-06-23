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

