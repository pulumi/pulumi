// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { Property, PropertyValue } from "../property";
import { Resource, URN } from "../resource";
import { Log } from "./log";
import { getMonitor, isDryRun } from "./settings";

// internalGetState fetches the internal state of a property for purposes of runtime use.
export function internalGetState<T>(property: Property<T>): PropertyState<T> {
    return (<any>property).state as PropertyState<T>;
}

// PropertyState is the internal representation of a resource's property state.  It is used by the runtime
// to resolve to final values and hook into important lifecycle states.
export class PropertyState<T> {
    public readonly inputPromise: Promise<T | undefined> | undefined; // a promise for this property's final input.
    public readonly outputPromise: Promise<T | undefined>; // a promise for this property's final output.

    private input: T | undefined; // the property's value, evolving until it becomes final.
    private output: T | undefined; // the property's output value, or undefined if it is unknown.
    private resolveInput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve input values.
    private resolveOutput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve output values.

    constructor(value?: PropertyValue<T>) {
        // Either link to a property or promise to resolve the input, or do it immediately, if it's available.
        if (value !== undefined) {
            this.inputPromise = new Promise<T | undefined>(
                (resolve: (v: T | undefined) => void) => { this.resolveInput = resolve; },
            );
            if (value instanceof Property) {
                internalGetState(value).outputPromise.then((v: T | undefined) => { this.setInput(v); });
            }
            else if (value instanceof Promise) {
                value.then((v: T) => { this.setInput(v); });
            }
            else {
                this.setInput(value);
            }
        }

        // We use different input and output promises, because we depend on the different values in different cases.
        this.outputPromise = new Promise<T | undefined>(
            (resolve: (v: T | undefined) => void) => { this.resolveOutput = resolve; },
        );
    }

    // inputValue returns the input value associated with this property, or undefined if it doesn't exist yet.
    public inputValue(): T | undefined {
        return this.input;
    }

    // outputValue returns the output value associated with this property, or undefined if it doesn't exist yet.
    public outputValue(): T | undefined {
        return this.output;
    }

    // then attaches a callback for the resolution of a computed value, and returns a newly computed value.
    public then<U>(callback: (v: T) => U): PropertyState<U> {
        let result = new PropertyState<U>();
        this.outputPromise.then((value: T | undefined) => {
            // If the value is unknown, propagate an unknown.  Otherwise, use the callback to compute something new.
            if (value === undefined) {
                result.setOutput(undefined, true, false);
            }
            else {
                result.setOutput(callback(value), true, false);
            }
        });
        return result;
    }

    // done marks the resolver as done, and prevents subsequent changes.  If it was initialized with a value, and no
    // new value has subsequently arrived, then that value is propagated as the final value.
    public done(dryRun: boolean): void {
        // If the value hasn't reached a final state yet, conditionally propagate its provisional value.  Note
        // that if we're still planning, we can't know that this state is final, and so we'll propagate undefined.
        let value: T | undefined;
        if (!dryRun) {
            value = this.output;
            if (value === undefined) {
                value = this.input;
            }
        }
        this.setOutput(value, true, true);
    }

    // setOutput resolves the final output value of a property.
    public setOutput(value: T | undefined, isFinal: boolean, skipIfAlready: boolean): void {
        if (this.resolveOutput === undefined) {
            if (!skipIfAlready) {
                throw new Error(`Illegal attempt to set a property output multiple times (${value})`);
            }
        }
        else {
            this.output = value;
            if (isFinal) {
                this.resolveOutput(value);
                this.resolveOutput = undefined;
            }
        }
    }

    // setInput resolves the initial input value of a property.
    private setInput(value: T | undefined): void {
        if (this.resolveInput === undefined) {
            throw new Error(`Illegal attempt to set a property input multiple times (${value})`);
        }
        this.input = value;
        this.resolveInput(value);
        this.resolveInput = undefined;
    }
}

