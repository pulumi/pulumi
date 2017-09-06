// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { Computed, MaybeComputed } from "../computed";
import { Resource, URN } from "../resource";
import { Log } from "./log";
import { getMonitor, isDryRun } from "./settings";

// Property is the internal representation of a resource's property state.  It is used by the runtime
// to resolve to final values and hook into important lifecycle states.
export class Property<T> implements Computed<T> {
    public readonly inputPromise: Promise<T | undefined> | undefined; // a promise for this property's final input.
    public readonly outputPromise: Promise<T | undefined>; // a promise for this property's final output.

    private input: T | undefined; // the property's value, evolving until it becomes final.
    private output: T | undefined; // the property's output value, or undefined if it is unknown.
    private resolveInput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve input values.
    private resolveOutput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve output values.

    constructor(value?: MaybeComputed<T>) {
        // Either link to a property or promise to resolve the input, or do it immediately, if it's available.
        if (value !== undefined) {
            this.inputPromise = new Promise<T | undefined>(
                (resolve: (v: T | undefined) => void) => { this.resolveInput = resolve; },
            );
            if (value instanceof Promise) {
                value.then((v: T) => { this.setInput(v); });
            }
            else if (value instanceof Property) {
                // For properties specifically, we want to intercept the value resolution and go straight
                // to the output promise because the implementation of map for property will propagate unknowns.
                value.outputPromise.then((v: T | undefined) => { this.setInput(v); });
            }
            else if ((value as Computed<T>).mapValue !== undefined) {
                // For all other computed properties, simply wire up the map and propagate the values.
                (value as Computed<T>).mapValue((v: T | undefined) => { this.setInput(v); });
            }
            else {
                this.setInput(value as T);
            }
        }

        // We use different input and output promises, because we depend on the different values in different cases.
        this.outputPromise = new Promise<T | undefined>(
            (resolve: (v: T | undefined) => void) => { this.resolveOutput = resolve; },
        );
    }

    // mapValue attaches a callback for the resolution of a computed value, and returns a newly computed value.
    public mapValue<U>(callback: (v: T) => MaybeComputed<U>): Computed<U> {
        let result = new Property<U>();
        this.outputPromise.then((value: T | undefined) => {
            // If the value is unknown, propagate an unknown.  Otherwise, use the callback to compute something new.
            if (value === undefined) {
                result.setOutput(undefined, true, false);
            }
            else {
                try {
                    // There's a callback; invoke it.
                    let u: MaybeComputed<U> = callback(value);

                    // If this is another computed, we need to wire up to its resolution; else just store the value.
                    if (u instanceof Promise) {
                        u.then((v: U) => { result.setOutput(v, true, false); });
                    }
                    else if ((u as Computed<U>).mapValue) {
                        (u as Computed<U>).mapValue((v: U) => {
                            result.setOutput(v, true, false);
                        });
                    }
                    else {
                        result.setOutput(<U>u, true, false);
                    }
                }
                catch (err) {
                    Log.error(`MapValue of a Computed yielded an unhandled error: ${err}`);
                }
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

    // toString overrides the standard toString to provide a "helpful" message.  Most likely this was a mistake,
    // and perhaps the message will help to indicate this, although sometimes it is helpful.
    public toString(): string {
        return `[pulumi-fabric Property: ` +
            `input=${this.input} output=${this.output} ` +
            `resolved=${this.resolveOutput === undefined}]`;
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

