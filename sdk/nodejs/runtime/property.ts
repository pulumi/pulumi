// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { Computed, MaybeComputed } from "../computed";
import { Resource, URN } from "../resource";
import { Log } from "./log";
import { getMonitor, isDryRun } from "./settings";

// mapValueCallbackRecursionCount tracks the recursion depth inside of mapValue callbacks.  This is used to
// prevent resource creation inside of such callbacks, as doing so leads to conditional resource creation.
let mapValueCallbackRecursionCount = 0;

// isInsideMapValueCallback is used by the runtime to ensure resources aren't conditionally created.
export function isInsideMapValueCallback(): boolean {
    return (mapValueCallbackRecursionCount > 0);
}

// Property is the internal representation of a resource's property state.  It is used by the runtime
// to resolve to final values and hook into important lifecycle states.
export class Property<T> implements Computed<T> {
    public readonly inputPromise: Promise<T | undefined>;  // a promise for this property's final input.
    public readonly outputPromise: Promise<T | undefined>; // a promise for this property's final output.

    private input: T | undefined; // the property's value, evolving until it becomes final.
    private output: T | undefined; // the property's output value, or undefined if it is unknown.
    private resolveInput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve input values.
    private resolveOutput: ((v: T | undefined) => void) | undefined; // the resolver used to resolve output values.

    // constructs a new property.  If immediate is true, the input state is used to resolve the final value
    // immediately.  Otherwise, input values will not lead to resolution of the final value, and some internal
    // logic will instead need to manually invoke setOutput in order to resolve it to a final value.
    constructor(value: MaybeComputed<T> | undefined, setInput: boolean, setOutput: boolean) {
        // We use different input and output promises, because we depend on the different values in different cases.
        this.inputPromise = new Promise<T | undefined>(
            (resolve: (v: T | undefined) => void) => { this.resolveInput = resolve; },
        );
        this.outputPromise = new Promise<T | undefined>(
            (resolve: (v: T | undefined) => void) => { this.resolveOutput = resolve; },
        );

        Property.resolveTo(this, value, setInput, setOutput);
    }

    private static resolveTo<T>(p: Property<T>, value: MaybeComputed<T> | undefined,
                                setInput: boolean, setOutput: boolean): void {
        // If this is another computed, we need to wire up to its resolution; else just store the value.
        if (value && value instanceof Promise) {
            value.then(
                (v: T | Promise<T>) => { Property.resolveTo(p, v, setInput, setOutput); },
                (err: Error) => { Log.error(`Unexpected error in dependent mapValue promise: ${err}`); },
            );
        }
        else if (value && value instanceof Property) {
            value.outputPromise.then(
                (v: T | undefined) => { Property.resolveTo(p, v, setInput, setOutput); },
                (err: Error) => { Log.error(`Unexpected error in dependent mapValue property`); },
            );
        }
        else if (value && (value as Computed<T>).mapValue) {
            (value as Computed<T>).mapValue(
                (v: MaybeComputed<T>) => { Property.resolveTo(p, v, setInput, setOutput); });
        }
        else {
            if (setInput) {
                p.setInput(<T>value);
            }
            if (setOutput) {
                p.setOutput(<T>value, true, false);
            }
        }
    }

    // mapValue attaches a callback for the resolution of a computed value, and returns a newly computed value.
    public mapValue<U>(callback: (v: T) => MaybeComputed<U>): Computed<U> {
        let result = new Property<U>(undefined, false, false);

        // Fire off a promise hinging on the target's output that will resolve the resulting property.
        let outcome: Promise<any> = this.outputPromise.then((value: T | undefined) => {
            // If the value is unknown, propagate an unknown.  Otherwise, use the callback to compute something new.
            if (value === undefined) {
                Property.resolveTo(result, undefined, true, true);
            }
            else {
                // There's a callback; invoke it and resolve the value.
                try {
                    mapValueCallbackRecursionCount++;
                    let transformed: MaybeComputed<U> = callback(value);
                    Property.resolveTo(result, transformed, true, true);
                }
                finally {
                    mapValueCallbackRecursionCount--;
                }
            }
        });

        // Ensure we log any errors.
        outcome.catch((err: Error) => {
            Log.error(`MapValue of a Computed yielded an unhandled error: ${err}`);
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

    // awaitingInput returns true if the input has yet to arrive.
    public awaitingInput(): boolean {
        return (this.resolveInput !== undefined);
    }

    // awaitingOutput returns true if the output has yet to arrive.
    public awaitingOutput(): boolean {
        return (this.resolveOutput !== undefined);
    }

    // setInput resolves the initial input value of a property.
    public setInput(value: T | undefined): void {
        if (!this.awaitingInput()) {
            throw new Error(`Illegal attempt to set a property input multiple times (${value})`);
        }
        else if (value instanceof Promise) {
            throw new Error(`Unexpected dependent promise value for property input`);
        }
        else if (value instanceof Property) {
            throw new Error(`Unexpected dependent property value for property input`);
        }
        this.input = value;
        this.resolveInput!(value);
        this.resolveInput = undefined;
    }

    // setOutput resolves the final output value of a property.
    public setOutput(value: T | undefined, isFinal: boolean, skipIfAlready: boolean): void {
        if (!this.awaitingOutput()) {
            if (!skipIfAlready) {
                throw new Error(`Illegal attempt to set a property output multiple times (${value})`);
            }
        }
        else {
            if (value instanceof Promise) {
                throw new Error(`Unexpected dependent promise value for property input`);
            }
            else if (value instanceof Property) {
                throw new Error(`Unexpected dependent property value for property input`);
            }
            this.output = value;
            if (isFinal) {
                this.resolveOutput!(value);
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
}

