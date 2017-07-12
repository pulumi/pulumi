// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

class C {}

// initialize empty maps of various kinds (implicit types).
let m1 = new Map<string, string>();
let m2 = new Map<string, number>();
let m3 = new Map<number, string>();
let m4 = new Map<string, C>();
let m5 = new Map<C, number>();
let m6 = new Map<C, string[]>();

// initialize empty maps of various kinds (explicit types).
let m11: Map<string, string> = new Map<string, string>();
let m12: Map<string, number> = new Map<string, number>();
let m13: Map<number, string> = new Map<number, string>();
let m14: Map<string, C> = new Map<string, C>();
let m15: Map<C, number> = new Map<C, number>();
let m16: Map<C, string[]> = new Map<C, string[]>();

// initialize maps using array constants.
let m7 = new Map<string, string>([
    [ "foo", "bar" ],
    [ "baz", "buz" ],
]);

