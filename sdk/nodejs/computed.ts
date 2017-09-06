// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Computed represents a variable whose value may not yet be known.  Not only may it not be known, but it may or
// may not become known at some point in the future.  The program should not depend on concrete values for correctness,
// and should only use the `map` function to create derived dataflow values.  For example, a logical value `v` may be
// transformed by `v+1`, and that resulting computed value may be then plugged into places expecting computed values
// (and so on and so forth).  Unlike promises, the values are guaranteed to *never* resolve under some circumstances,
// such as during planning, so forward progress must not depend on resolution.
export interface Computed<T> {
    // mapValue attaches a callback for the resolution of a computed value, and returns a newly computed value.
    mapValue<U>(callback: (v: T) => MaybeComputed<U>): Computed<U>;
}

// MaybeComputed is one of: a T, a computed of T (whose value may not yet be known), or a promise of T.
export type MaybeComputed<T> = T | Computed<T> | Promise<T>;

