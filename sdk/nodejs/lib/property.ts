// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Property represents a variable whose value may not yet be known.  Not only may it not be known, but it may or
// may not become known at some point in the future.  The program should not depend on concrete values for correctness,
// and should only use the `then` function to create derived dataflow values.  For example, a logical value `v` may be
// transformed by `v+1`, and that resulting computed value may be then plugged into places expecting computed values
// (and so on and so forth).  Unlike promises, the values are guaranteed to *never* resolve under some circumstances,
// such as during planning, so forward progress must not depend on resolution.
export class Property<T> {
    private v?: T; // the value, if it exists.
    private link: Promise<T>; // non-undefined if this is linked to another property or promise.
    private promise: Promise<T>; // the underlying promises, for unresolved values.
    private resolver: ((v?: T | PromiseLike<T>) => void) | undefined; // the resolver used to resolve values.

    constructor(value?: PropertyValue<T>) {
        // No matter what, we will use a promise to resolve this to a final value later on.  This is true even
        // if a value was provided at construction time, because its value may change before settling.
        this.promise = new Promise<T>(
            (resolve: (v?: T | PromiseLike<T>) => void, reject: (r?: any) => void) => {
                this.resolver = resolve;
            },
        );

        // If this is linked to another Property or Promise, record this fact.
        if (value !== undefined) {
            if (value instanceof Property) {
                this.link = value.promise;
            }
            else if (value instanceof Promise) {
                this.link = value;
            }
            else {
                this.v = value;
            }
        }

        // Now ensure that we automatically propagate values for linked properties.
        if (this.link) {
            this.link.then((v: T) => {
                // Only propagate the value if another final value hasn't already been recorded.
                if (this.resolver) {
                    this.resolve(v);
                }
            });
        }
    }

    // linked returns the underlying promise this value is linked to, if any.
    public linked(): Promise<T> | undefined {
        return this.link;
    }

    // has returns true if this attribute has a value associated with it.
    public has(): boolean {
        return this.v !== undefined;
    }

    // require ensures that a value exists and returns it.  This function should be used with great care, because
    // values do not settle during planning, and this will fail; using this function can lead to brittle code.
    public require(): T {
        if (this.v === undefined) {
            throw new Error("Cannot get a property whose value is pending; use then");
        }
        return this.v;
    }

    // done marks the resolver as done, and prevents subsequent changes.  If it was initialized with a value, and no
    // new value has subsequently arrived, then that value is propagated as the final value.
    public done(): void {
        if (this.resolver && this.v !== undefined) {
            this.resolve(this.v);
        }
    }

    // resolve resolves the value of a property that was created out of thin air.
    public resolve(value: T): void {
        if (!this.resolver) {
            throw new Error("Illegal attempt to resolve a property value with a different origin");
        }
        this.v = value;
        this.resolver(value);
        this.resolver = undefined;
    }

    // sample returns the current value of the computed property, if it exists.  Note that this will differ between
    // planning and deployment executions of the same program, and may be subject to timing races.  Use of it is
    // generally discouraged except for diagnostics that may require knowing its value at given times.
    public sample(): T | undefined {
        return this.v;
    }

    // then attaches a callback for the resolution of a computed value, and returns a newly computed value.
    public then<U>(callback: (value: T) => U): Property<U> {
        let result = new Property<U>();
        this.promise.then((v: T) => { result.resolve(callback(v)); });
        return result;
    }
}

// PropertyValue is either a T, a "property value" of T (whose value may not yet be known), or a promise of T.
export type PropertyValue<T> = T | Property<T> | Promise<T> | undefined;

