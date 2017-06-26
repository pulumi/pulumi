// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is a simple test case that exports a class with a number of interesting facets: namely, a constructor, some
// member variables, and some member methods.

export class Point {
    public readonly x: number;
    public readonly y: number;

    constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
    }

    public add(other: Point): Point {
        return new Point(this.x + other.x, this.y + other.y);
    }
}

