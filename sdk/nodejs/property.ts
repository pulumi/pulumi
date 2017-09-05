// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { PropertyState } from "./runtime";

// Property represents a variable whose value may not yet be known.  Not only may it not be known, but it may or
// may not become known at some point in the future.  The program should not depend on concrete values for correctness,
// and should only use the `then` function to create derived dataflow values.  For example, a logical value `v` may be
// transformed by `v+1`, and that resulting computed value may be then plugged into places expecting computed values
// (and so on and so forth).  Unlike promises, the values are guaranteed to *never* resolve under some circumstances,
// such as during planning, so forward progress must not depend on resolution.
export class Property<T> {
    private state: PropertyState<T>; // the property state, managed by the runtime.

    constructor(value?: PropertyValue<T>, empty?: boolean) {
        if (!empty) {
            this.state = new PropertyState<T>(value);
        }
    }

    // has returns true if this attribute has a value associated with it.
    public has(): boolean {
        return this.sample() !== undefined;
    }

    // require ensures that a value exists and returns it.  This function should be used with great care, because
    // values do not settle during planning, and this will fail; using this function can lead to brittle code.
    public require(): T {
        let value: T | undefined = this.sample();
        if (value === undefined) {
            throw new Error("Property has no value associated with it; try using then");
        }
        return value;
    }

    // sample returns the current value of the computed property, if it exists.  Note that this will differ between
    // planning and deployment executions of the same program, and may be subject to timing races.  Use of it is
    // generally discouraged except for diagnostics that may require knowing its value at given times.
    public sample(): T | undefined {
        let value: T | undefined = this.state.outputValue();
        if (value === undefined) {
            value = this.state.inputValue();
        }
        return value;
   }

    // then attaches a callback for the resolution of a computed value, and returns a newly computed value.
    public then<U>(callback: (v: T) => U): Property<U> {
        let result = new Property<U>(undefined, true);
        result.state = this.state.then(callback);
        return result;
    }
}

// PropertyValue is either a T, a "property value" of T (whose value may not yet be known), or a promise of T.
export type PropertyValue<T> = T | Property<T> | Promise<T>;

