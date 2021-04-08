// Copyright 2016-2018, Pulumi Corporation.
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

import { Resource } from "./resource";
import * as runtime from "./runtime";
import * as utils from "./utils";

/*tslint:disable:no-shadowed-variable*/

/**
 * Output helps encode the relationship between Resources in a Pulumi application. Specifically an
 * Output holds onto a piece of Data and the Resource it was generated from. An Output value can
 * then be provided when constructing new Resources, allowing that new Resource to know both the
 * value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be created, which properly tracks the relationship between resources.
 */
class OutputImpl<T> implements OutputInstance<T> {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * This is internal instead of being truly private, to support mixins and our serialization model.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiOutput: boolean = true;

    /**
     * Whether or not this 'Output' wraps a secret value. Values which are marked as secret are stored in an
     * encrypted format when they are persisted as part of a state file. When`true` this "taints" any
     * additional resources created from it via an [all] or [apply], such that they are also treated as
     * secrets.
     * @internal
     */
    public readonly isSecret: Promise<boolean>;

    /**
     * Whether or not this 'Output' should actually perform .apply calls.  During a preview,
     * an Output value may not be known (because it would have to actually be computed by doing an
     * 'update').  In that case, we don't want to perform any .apply calls as the callbacks
     * may not expect an undefined value.  So, instead, we just transition to another Output
     * value that itself knows it should not perform .apply calls.
     * @internal
     */
    public readonly isKnown: Promise<boolean>;

    /**
     * Method that actually produces the concrete value of this output, as well as the total
     * deployment-time set of resources this output depends on. If the value of the output is not
     * known (i.e. isKnown resolves to false), this promise should resolve to undefined unless the
     * `withUnknowns` flag is passed, in which case it will resolve to `unknown`.
     *
     * Only callable on the outside.
     * @internal
     */
    public readonly promise: (withUnknowns?: boolean) => Promise<T>;

    /**
     * The list of resources that this output value depends on.
     *
     * Only callable on the outside.
     *
     * This only returns the set of dependent resources that were known at Output construction time.
     * It represents the `@pulumi/pulumi` api prior to the addition of 'async resource'
     * dependencies.  Code inside @pulumi/pulumi should use `.allResources` instead.
     * @internal
     */
    public readonly resources: () => Set<Resource>;

    /**
     * The entire list of resources that this output depends on.
     *
     * This includes both the dependent resources that were known when the Output was explicitly
     * instantiated, along with any dependent resources produced asynchronously and returned from
     * the function passed to `Output.apply`.
     *
     * This should be used whenever available inside this package.  However, code that uses this
     * should be resilient to it being absent and should fall back to using `.resources()` instead.
     *
     * Note: it is fine to use this property if it is guaranteed that it is on an output produced by
     * this SDK (and not another sxs version).
     * @internal
     */
    // Marked as optional for sxs scenarios.
    public readonly allResources?: () => Promise<Set<Resource>>;

    /**
     * [toString] on an [Output<T>] is not supported.  This is because the value an [Output] points
     * to is asynchronously computed (and thus, this is akin to calling [toString] on a [Promise]).
     *
     * Calling this will simply return useful text about the issue, and will log a warning. In a
     * future version of `@pulumi/pulumi` this will be changed to throw an error when this occurs.
     *
     * To get the value of an Output<T> as an Output<string> consider either:
     * 1. `o.apply(v => ``prefix${v}suffix``)` or
     * 2. `pulumi.interpolate ``prefix${v}suffix`` `
     *
     * This will return an Output with the inner computed value and all resources still tracked. See
     * https://pulumi.io/help/outputs for more details
     * @internal
     */
    public toString: () => string;

    /**
     * [toJSON] on an [Output<T>] is not supported.  This is because the value an [Output] points
     * to is asynchronously computed (and thus, this is akin to calling [toJSON] on a [Promise]).
     *
     * Calling this will simply return useful text about the issue, and will log a warning. In a
     * future version of `@pulumi/pulumi` this will be changed to throw an error when this occurs.
     *
     * To get the value of an Output as a JSON value or JSON string consider either:
     * 1. `o.apply(v => v.toJSON())` or
     * 2. `o.apply(v => JSON.stringify(v))`
     *
     * This will return an Output with the inner computed value and all resources still tracked.
     * See https://pulumi.io/help/outputs for more details
     * @internal
     */
    public toJSON: () => any;

    // Statics

    /**
     * create takes any Input value and converts it into an Output, deeply unwrapping nested Input
     * values as necessary.
     */
    public static create<T>(val: Input<T>): Output<Unwrap<T>>;
    public static create<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;
    public static create<T>(val: Input<T | undefined>): Output<Unwrap<T | undefined>> {
        return output(val);
    }

    /**
     * Returns true if the given object is an instance of Output<T>.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance<T>(obj: any): obj is Output<T> {
        return utils.isInstance(obj, "__pulumiOutput");
    }

    /** @internal */
    static async getPromisedValue<T>(promise: Promise<T>, withUnknowns?: boolean): Promise<T> {
        // If the caller did not explicitly ask to see unknown values and val contains unknowns, return undefined. This
        // preserves compatibility with earlier versions of the Pulumi SDK.
        const val = await promise;
        if (!withUnknowns && containsUnknowns(val)) {
            return <T><any>undefined;
        }
        return val;
    }

    /** @internal */
    public constructor(
            resources: Set<Resource> | Resource[] | Resource,
            promise: Promise<T>,
            isKnown: Promise<boolean>,
            isSecret: Promise<boolean>,
            allResources: Promise<Set<Resource> | Resource[] | Resource> | undefined) {

        // Always create a copy so that no one accidentally modifies our Resource list.
        const resourcesCopy = copyResources(resources);

        // Create a copy of the async resources.  Populate this with the sync-resources if that's
        // all we have.  That way this is always ensured to be a superset of the list of sync resources.
        allResources = allResources || Promise.resolve([]);
        const allResourcesCopy = allResources.then(r => utils.union(copyResources(r), resourcesCopy));

        // We are only known if we are not explicitly unknown and the resolved value of the output
        // contains no distinguished unknown values.
        isKnown = Promise.all([isKnown, promise]).then(([known, val]) => known && !containsUnknowns(val));

        const lifted = Promise.all([allResourcesCopy, promise, isKnown, isSecret])
                              .then(([liftedResources, value, liftedIsKnown, liftedIsSecret]) => liftInnerOutput(liftedResources, value, liftedIsKnown, liftedIsSecret));

        this.resources = () => resourcesCopy;
        this.allResources = () => lifted.then(l => l.allResources);

        this.isKnown = lifted.then(l => l.isKnown);
        this.isSecret = lifted.then(l => l.isSecret);
        this.promise = (withUnknowns?: boolean) => OutputImpl.getPromisedValue(lifted.then(l => l.value), withUnknowns);

        this.toString = () => {
            const message =
`Calling [toString] on an [Output<T>] is not supported.

To get the value of an Output<T> as an Output<string> consider either:
1: o.apply(v => \`prefix\${v}suffix\`)
2: pulumi.interpolate \`prefix\${v}suffix\`

See https://pulumi.io/help/outputs for more details.
This function may throw in a future version of @pulumi/pulumi.`;
            return message;
        };

        this.toJSON = () => {
            const message =
`Calling [toJSON] on an [Output<T>] is not supported.

To get the value of an Output as a JSON value or JSON string consider either:
    1: o.apply(v => v.toJSON())
    2: o.apply(v => JSON.stringify(v))

See https://pulumi.io/help/outputs for more details.
This function may throw in a future version of @pulumi/pulumi.`;
            return message;
        };

        return new Proxy(this, {
            get: (obj, prop: keyof T) => {
                // Recreate the prototype walk to ensure we find any actual members defined directly
                // on `Output<T>`.
                for (let o = obj; o; o = Object.getPrototypeOf(o)) {
                    if (o.hasOwnProperty(prop)) {
                        return (<any>o)[prop];
                    }
                }

                // Always explicitly fail on a member called 'then'.  It is used by other systems to
                // determine if this is a Promise, and we do not want to indicate that that's what
                // we are.
                if (prop === "then") {
                    return undefined;
                }

                // Do not lift members that start with __.  Technically, if all libraries were
                // using this version of pulumi/pulumi we would not need this.  However, this is
                // so that downstream consumers can use this version of pulumi/pulumi while also
                // passing these new Outputs to older versions of pulumi/pulumi.  The reason this
                // can be a problem is that older versions do an RTTI check that simply asks questions
                // like:
                //
                //      Is there a member on this object called '__pulumiResource'
                //
                // If we automatically lift such a member (even if it eventually points to 'undefined'),
                // then those RTTI checks will succeed.
                //
                // Note: this should be safe to not lift as, in general, properties with this prefix
                // are not at all common (and in general are used to represent private things anyway
                // that likely should not be exposed).
                //
                // Similarly, do not respond to the 'doNotCapture' member name.  It serves a similar
                // RTTI purpose.
                if (typeof prop === "string") {
                    if (prop.startsWith("__") || prop === "doNotCapture" || prop === "deploymentOnlyModule") {
                        return undefined;
                    }
                }

                // Fail out if we are being accessed using a symbol.  Many APIs will access with a
                // well known symbol (like 'Symbol.toPrimitive') to check for the presence of something.
                // They will only check for the existence of that member, and we don't want to make it
                // appear that have those.
                //
                // Another way of putting this is that we only forward 'string/number' members to our
                // underlying value.
                if (typeof prop === "symbol") {
                    return undefined;
                }

                // Else for *any other* property lookup, succeed the lookup and return a lifted
                // `apply` on the underlying `Output`.
                return (<any>obj.apply)((ob: any) => {
                    if (ob === undefined || ob === null) {
                        return undefined;
                    }
                    else if (isUnknown(ob)) {
                        // If the value of this output is unknown, the result of the access should also be unknown.
                        // This is conceptually consistent, and also prevents us from returning a "known undefined"
                        // value from the `ob[prop]` expression below.
                        return unknown;
                    }

                    return ob[prop];
                }, /*runWithUnknowns:*/ true);
            },
        });
    }

    public get(): T {
        throw new Error(`Cannot call '.get' during update or preview.
To manipulate the value of this Output, use '.apply' instead.`);
    }

    // runWithUnknowns requests that `func` is run even if `isKnown` resolves to `false`. This is used to allow
    // callers to process fully- or partially-unknown values and return a known result. the output proxy takes
    // advantage of this to allow proxied property accesses to return known values even if other properties of
    // the containing object are unknown.
    public apply<U>(func: (t: T) => Input<U>, runWithUnknowns?: boolean): Output<U> {
        // we're inside the modern `output` code, so it's safe to call `.allResources!` here.

        const applied = Promise.all([this.allResources!(), this.promise(/*withUnknowns*/ true), this.isKnown, this.isSecret])
                               .then(([allResources, value, isKnown, isSecret]) => applyHelperAsync<T, U>(allResources, value, isKnown, isSecret, func, !!runWithUnknowns));

        const result = new OutputImpl<U>(
            this.resources(),
            applied.then(a => a.value),
            applied.then(a => a.isKnown),
            applied.then(a => a.isSecret),
            applied.then(a => a.allResources));
        return <Output<U>><any>result;
    }
}

/** @internal */
export function getAllResources<T>(op: OutputInstance<T>): Promise<Set<Resource>> {
    return op.allResources instanceof Function
        ? op.allResources()
        : Promise.resolve(op.resources());
}

function copyResources(resources: Set<Resource> | Resource[] | Resource) {
    const copy = Array.isArray(resources) ? new Set(resources) :
                 resources instanceof Set ? new Set(resources) :
                 new Set([resources]);
    return copy;
}

async function liftInnerOutput(allResources: Set<Resource>, value: any, isKnown: boolean, isSecret: boolean) {
    if (!Output.isInstance(value)) {
        // 'value' itself wasn't an output, no need to transform any of the data we got.
        return { allResources, value, isKnown, isSecret };
    }

    // 'value' was an Output.  So we unwrap that to get the inner value/isKnown/isSecret/resources
    // returned by that Output and merge with the state passed in to get the state of the final Output.

    // Note: we intentionally await all the promises of the inner output. This way we properly
    // propagate any rejections of any of these promises through the outer output as well.
    const innerValue = await value.promise(/*withUnknowns*/ true);
    const innerIsKnown = await value.isKnown;
    const innerIsSecret = await (value.isSecret || Promise.resolve(false));

    // If we're working with a new-style output, grab all its resources and merge into ours.
    // Otherwise, if this is an old-style output, just grab the resources it was known to have
    // at construction time.
    const innerResources = await getAllResources(value);
    const totalResources = utils.union(allResources, innerResources);
    return {
        allResources: totalResources,
        value: innerValue,
        isKnown: innerIsKnown,
        isSecret: isSecret || innerIsSecret,
    };
}

// tslint:disable:max-line-length
async function applyHelperAsync<T, U>(
        allResources: Set<Resource>, value: T, isKnown: boolean, isSecret: boolean,
        func: (t: T) => Input<U>, runWithUnknowns: boolean) {
    if (runtime.isDryRun()) {
        // During previews only perform the apply if the engine was able to give us an actual value
        // for this Output.
        const applyDuringPreview = isKnown || runWithUnknowns;

        if (!applyDuringPreview) {
            // We didn't actually run the function, our new Output is definitely **not** known.
            return {
                allResources,
                value: <U><any>undefined,
                isKnown: false,
                isSecret,
            };
        }

        // If we are running with unknown values and the value is explicitly unknown but does not actually
        // contain any unknown values, collapse its value to the unknown value. This ensures that callbacks
        // that expect to see unknowns during preview in outputs that are not known will always do so.
        if (!isKnown && runWithUnknowns && !containsUnknowns(value)) {
            value = <T><any>unknown;
        }
    }

    const transformed = await func(value);

    // We successfully ran the inner function. Our new Output should be considered known.  We
    // preserve secretness from our original Output to the new one we're creating.
    return liftInnerOutput(allResources, transformed, /*isKnown*/ true, isSecret);
}

// Returns an promise denoting if the output is a secret or not. This is not the same as just calling `.isSecret`
// because in cases where the output does not have a `isSecret` property and it is a Proxy, we need to ignore
// the isSecret member that the proxy reports back.
// This calls the public implementation so that we only make any calculations in a single place.
/** @internal */
export function isSecretOutput<T>(o: Output<T>): Promise<boolean> {
    return isSecret(o);
}

// Helper function for `output`.  This function trivially recurses through an object, copying it,
// while also lifting any inner Outputs (with all their respective state) to a top-level Output at
// the end.  If there are no inner outputs, this will not affect the data (except by producing a new
// copy of it).
//
// Importantly:
//
//  1. Resources encountered while recursing are not touched.  This helps ensure they stay Resources
//     (with an appropriate prototype chain).
//  2. Primitive values (string, number, etc.) are returned as is.
//  3. Arrays and Record are recursed into.  An Array<...> that contains any Outputs wil become an
//     Output<Array<Unwrapped>>.  A Record<string, ...> that contains any Output values will be an
//     Output<Record<string, Unwrap<...>>.  In both cases of recursion, the outer Output's
//     known/secret/resources will be computed from the nested Outputs.
function outputRec(val: any): any {
    if (val === null || typeof val !== "object") {
        // strings, numbers, booleans, functions, symbols, undefineds, nulls are all returned as
        // themselves.  They are always 'known' (i.e. we can safely 'apply' off of them even during
        // preview).
        return val;
    }
    else if (Resource.isInstance(val)) {
        // Don't unwrap Resources, there are existing codepaths that return Resources through
        // Outputs and we want to preserve them as is when flattening.
        return val;
    }
    else if (isUnknown(val)) {
        return val;
    }
    else if (val instanceof Promise) {
        // Recurse into the value the Promise points to.  This may end up producing a
        // Promise<Output>. Wrap this in another Output as the final result.  This Output's
        // construction will be able to merge the inner Output's data with its own.  See
        // liftInnerOutput for more details.
        return createSimpleOutput(val.then(v => outputRec(v)));
    }
    else if (Output.isInstance(val)) {
        // We create a new output here from the raw pieces of the original output in order to
        // accommodate outputs from downlevel SxS SDKs.  This ensures that within this package it is
        // safe to assume the implementation of any Output returned by the `output` function.
        //
        // This includes:
        // 1. that first-class unknowns are properly represented in the system: if this was a
        //    downlevel output where val.isKnown resolves to false, this guarantees that the
        //    returned output's promise resolves to unknown.
        // 2. That the `isSecret` property is available.
        // 3. That the `.allResources` is available.
        const allResources = getAllResources(val);
        const newOutput = new OutputImpl(
            val.resources(), val.promise(/*withUnknowns*/ true), val.isKnown, val.isSecret, allResources);
        return newOutput.apply(outputRec, /*runWithUnknowns*/ true);
    }
    else if (val instanceof Array) {
        const allValues = [];
        let hasOutputs = false;
        for (const v of val) {
            const ev = outputRec(v);

            allValues.push(ev);
            if (Output.isInstance(ev)) {
                hasOutputs = true;
            }
        }

        // If we didn't encounter any nested Outputs, we don't need to do anything.  We can just
        // return this value as is.
        if (!hasOutputs) {
            // Note: we intentionally return 'allValues' here and not 'val'.  This ensures we get a
            // copy.  This has been behavior we've had since the beginning and there may be subtle
            // logic out there that depends on this that we would not want ot break.
            return allValues;
        }

        // Otherwise, combine the data from all the outputs/non-outputs to one final output.
        const promisedArray = Promise.all(allValues.map(v => getAwaitableValue(v)));
        const [syncResources, isKnown, isSecret, allResources] = getResourcesAndDetails(allValues);
        return new Output(syncResources, promisedArray, isKnown, isSecret, allResources);
    }
    else {
        const promisedValues: { key: string; value: any }[] = [];
        let hasOutputs = false;
        for (const k of Object.keys(val)) {
            const ev = outputRec(val[k]);

            promisedValues.push({ key: k, value: ev });
            if (Output.isInstance(ev)) {
                hasOutputs = true;
            }
        }

        if (!hasOutputs) {
            // Note: we intentionally return a new value here and not 'val'.  This ensures we get a
            // copy.  This has been behavior we've had since the beginning and there may be subtle
            // logic out there that depends on this that we would not want ot break.
            return promisedValues.reduce((o, kvp) => { o[kvp.key] = kvp.value; return o; }, <any>{});
        }

        const promisedObject = getPromisedObject(promisedValues);
        const [syncResources, isKnown, isSecret, allResources] = getResourcesAndDetails(promisedValues.map(kvp => kvp.value));
        return new Output(syncResources, promisedObject, isKnown, isSecret, allResources);
    }
}

/**
 * [output] takes any Input value and converts it into an Output, deeply unwrapping nested Input
 * values as necessary.
 *
 * The expected way to use this function is like so:
 *
 * ```ts
 *      var transformed = pulumi.output(someVal).apply(unwrapped => {
 *          // Do whatever you want now.  'unwrapped' will contain no outputs/promises inside
 *          // here, so you can easily do whatever sort of transformation is most convenient.
 *      });
 *
 *      // the result can be passed to another Resource.  The dependency information will be
 *      // properly maintained.
 *      var someResource = new SomeResource(name, { data: transformed ... });
 * ```
 */
export function output<T>(val: Input<T>): Output<Unwrap<T>>;
export function output<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;
export function output<T>(val: Input<T | undefined>): Output<Unwrap<T | undefined>> {
    const ov = outputRec(val);
    return Output.isInstance<Unwrap<T>>(ov) ? ov : createSimpleOutput(ov);
}

/**
 * [secret] behaves the same as [output] except the returned output is marked as containing sensitive data.
 */
export function secret<T>(val: Input<T>): Output<Unwrap<T>>;
export function secret<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;
export function secret<T>(val: Input<T | undefined>): Output<Unwrap<T | undefined>> {
    const o = output(val);

    // we called `output` right above this, so it's safe to call `.allResources` on the result.
    return new Output(
        o.resources(), o.promise(/*withUnknowns*/ true),
        o.isKnown, Promise.resolve(true), o.allResources!());
}

/**
 * [unsecret] behaves the same as [output] except the returned output takes the existing output and unwraps the secret
 */
export function unsecret<T>(val: Output<T>): Output<T> {
   return new Output(
        val.resources(), val.promise(/*withUnknowns*/ true),
        val.isKnown, Promise.resolve(false), val.allResources!());
}

export function isSecret<T>(val: Output<T>): Promise<boolean> {
    return Output.isInstance(val.isSecret) ? Promise.resolve(false) : val.isSecret;
}

function createSimpleOutput(val: any) {
    return new Output(
        new Set(),
        val instanceof Promise ? val : Promise.resolve(val),
        /*isKnown*/ Promise.resolve(true),
        /*isSecret */ Promise.resolve(false),
        Promise.resolve(new Set()));
}

/**
 * Allows for multiple Output objects to be combined into a single Output object.  The single Output
 * will depend on the union of Resources that the individual dependencies depend on.
 *
 * This can be used in the following manner:
 *
 * ```ts
 * var d1: Output<string>;
 * var d2: Output<number>;
 *
 * var d3: Output<ResultType> = Output.all([d1, d2]).apply(([s, n]) => ...);
 * ```
 *
 * In this example, taking a dependency on d3 means a resource will depend on all the resources of
 * d1 and d2.
 */
// tslint:disable:max-line-length
export function all<T>(val: Record<string, Input<T>>): Output<Record<string, Unwrap<T>>>;
export function all<T1, T2, T3, T4, T5, T6, T7, T8>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined, Input<T7> | undefined, Input<T8> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>, Unwrap<T4>, Unwrap<T5>, Unwrap<T6>, Unwrap<T7>, Unwrap<T8>]>;
export function all<T1, T2, T3, T4, T5, T6, T7>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined, Input<T7> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>, Unwrap<T4>, Unwrap<T5>, Unwrap<T6>, Unwrap<T7>]>;
export function all<T1, T2, T3, T4, T5, T6>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>, Unwrap<T4>, Unwrap<T5>, Unwrap<T6>]>;
export function all<T1, T2, T3, T4, T5>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>, Unwrap<T4>, Unwrap<T5>]>;
export function all<T1, T2, T3, T4>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>, Unwrap<T4>]>;
export function all<T1, T2, T3>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>, Unwrap<T3>]>;
export function all<T1, T2>(values: [Input<T1> | undefined, Input<T2> | undefined]): Output<[Unwrap<T1>, Unwrap<T2>]>;
export function all<T>(ds: (Input<T> | undefined)[]): Output<Unwrap<T>[]>;
export function all<T>(val: Input<T>[] | Record<string, Input<T>>): Output<any> {
    // Our recursive `output` helper already does exactly what `all` needs to do in terms of the
    // implementation. Why have both `output` and `all` then?  Currently, to the best of our
    // abilities, we haven't been able to make a single signature for both that can unify tuples and
    // arrays for TypeScript.  So `all` is much better when dealing with a tuple of heterogenous
    // values, while `output` is good for everything else.
    //
    // Specifically ``all` can take an `[Output<string>, Output<number>]` and produce an
    // `Output<[string, number]>` However, `output` for that same type will produce an
    // `Output<(string|number)[]>` which is definitely suboptimal.
    return output(val);
}

function getAwaitableValue(v: any): any {
    if (Output.isInstance(v)) {
        return v.promise(/* withUnknowns */ true);
    } else {
        return v;
    }
}

async function getPromisedObject<T>(
        keysAndOutputs: { key: string; value: any }[]): Promise<Record<string, Unwrap<T>>> {
    const result: Record<string, Unwrap<T>> = {};
    for (const kvp of keysAndOutputs) {
        result[kvp.key] = await getAwaitableValue(kvp.value);
    }

    return result;
}

function getResourcesAndDetails(allValues: any[]): [Set<Resource>, Promise<boolean>, Promise<boolean>, Promise<Set<Resource>>] {
    const syncResources = new Set<Resource>();
    const allOutputs = [];
    for (const v of allValues) {
        if (Output.isInstance(v)) {
            allOutputs.push(v);
            for (const res of v.resources()) {
                syncResources.add(res);
            }
        }
    }

    // All the outputs were generated in `function all` using `output(v)`.  So it's safe
    // to call `.allResources!` here.
    const allResources = Promise.all(allOutputs.map(o => o.allResources!())).then(arr => {
        const result = new Set<Resource>();

        for (const set of arr) {
            for (const res of set) {
                result.add(res);
            }
        }

        return result;
    });

    // A merged output is known if all of its inputs are known.
    const isKnown = Promise.all(allOutputs.map(o => o.isKnown)).then(ps => ps.every(b => b));

    // A merged output is secret if any of its inputs are secret.
    const isSecret = Promise.all(allOutputs.map(o => isSecretOutput(o))).then(ps => ps.some(b => b));

    return [syncResources, isKnown, isSecret, allResources];
}

/**
 * Unknown represents a value that is unknown. These values correspond to unknown property values received from the
 * Pulumi engine as part of the result of a resource registration (see runtime/rpc.ts). User code is not typically
 * exposed to these values: any Output<> that contains an Unknown will itself be unknown, so any user callbacks
 * passed to `apply` will not be run. Internal callers of `apply` can request that they are run even with unknown
 * values; the output proxy takes advantage of this to allow proxied property accesses to return known values even
 * if other properties of the containing object are unknown.
 */
class Unknown {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * This is internal instead of being truly private, to support mixins and our serialization model.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiUnknown: boolean = true;

    /**
     * Returns true if the given object is an instance of Unknown. This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is Unknown {
        return utils.isInstance<Unknown>(obj, "__pulumiUnknown");
    }
}

/**
 * unknown is the singleton unknown value.
 * @internal
 */
export const unknown = new Unknown();

/**
 * isUnknown returns true if the given value is unknown.
 */
export function isUnknown(val: any): boolean {
    return Unknown.isInstance(val);
}

/**
 * containsUnknowns returns true if the given value is or contains unknown values.
 */
export function containsUnknowns(value: any): boolean {
    return impl(value, new Set<any>());

    function impl(val: any, seen: Set<any>): boolean {
        if (val === null || typeof val !== "object") {
            return false;
        }
        else if (isUnknown(val)) {
            return true;
        }
        else if (seen.has(val)) {
            return false;
        }

        seen.add(val);
        if (val instanceof Array) {
            return val.some(e => impl(e, seen));
        }
        else {
            return Object.keys(val).some(k => impl(val[k], seen));
        }
    }
}

/**
 * [Input] is a property input for a resource.  It may be a promptly available T, a promise for one,
 * or the output from a existing Resource.
 */
// Note: we accept an OutputInstance (and not an Output) here to be *more* flexible in terms of
// what an Input is.  OutputInstance has *less* members than Output (because it doesn't lift anything).
export type Input<T> = T | Promise<T> | OutputInstance<T>;

/**
 * [Inputs] is a map of property name to property input, one for each resource property value.
 */
export type Inputs = Record<string, Input<any>>;

/**
 * The 'Unwrap' type allows us to express the operation of taking a type, with potentially deeply
 * nested Promises and Outputs and to then get that same type with all the Promises and Outputs
 * replaced with their wrapped type.  Note that this Unwrapping is 'deep'.  So if you had:
 *
 *      `type X = { A: Promise<{ B: Output<{ c: Input<boolean> }> }> }`
 *
 * Then `Unwrap<X>` would be equivalent to:
 *
 *      `...    = { A: { B: { C: boolean } } }`
 *
 * Unwrapping sees through Promises, Outputs, Arrays and Objects.
 *
 * Note: due to TypeScript limitations there are some things that cannot be expressed. Specifically,
 * if you had a `Promise<Output<T>>` then the Unwrap type would not be able to undo both of those
 * wraps. In practice that should be ok.  Values in an object graph should not wrap Outputs in
 * Promises.  Instead, any code that needs to work Outputs and also be async should either create
 * the Output with the Promise (which will collapse into just an Output).  Or, it should start with
 * an Output and call [apply] on it, passing in an async function.  This will also collapse and just
 * produce an Output.
 *
 * In other words, this should not be used as the shape of an object: `{ a: Promise<Output<...>> }`.
 * It should always either be `{ a: Promise<NonOutput> }` or just `{ a: Output<...> }`.
 */
export type Unwrap<T> =
    // 1. If we have a promise, just get the type it itself is wrapping and recursively unwrap that.
    // 2. Otherwise, if we have an output, do the same as a promise and just unwrap the inner type.
    // 3. Otherwise, we have a basic type.  Just unwrap that.
    T extends Promise<infer U1> ? UnwrapSimple<U1> :
    T extends OutputInstance<infer U2> ? UnwrapSimple<U2> :
    UnwrapSimple<T>;

type primitive = Function | string | number | boolean | undefined | null;

/**
 * Handles encountering basic types when unwrapping.
 */
export type UnwrapSimple<T> =
    // 1. Any of the primitive types just unwrap to themselves.
    // 2. An array of some types unwraps to an array of that type itself unwrapped. Note, due to a
    //    TS limitation we cannot express that as Array<Unwrap<U>> due to how it handles recursive
    //    types. We work around that by introducing an structurally equivalent interface that then
    //    helps make typescript defer type-evaluation instead of doing it eagerly.
    // 3. An object unwraps to an object with properties of the same name, but where the property
    //    types have been unwrapped.
    // 4. return 'never' at the end so that if we've missed something we'll discover it.
    T extends primitive ? T :
    T extends Resource ? T :
    T extends Array<infer U> ? UnwrappedArray<U> :
    T extends object ? UnwrappedObject<T> :
    never;

export interface UnwrappedArray<T> extends Array<Unwrap<T>> {}

export type UnwrappedObject<T> = {
    [P in keyof T]: Unwrap<T[P]>;
};

/**
 * Instance side of the [Output<T>] type.  Exposes the deployment-time and run-time mechanisms
 * for working with the underlying value of an [Output<T>].
 */
export interface OutputInstance<T> {
    /** @internal */ allResources?: () => Promise<Set<Resource>>;
    /** @internal */ readonly isKnown: Promise<boolean>;
    /** @internal */ readonly isSecret: Promise<boolean>;
    /** @internal */ promise(withUnknowns?: boolean): Promise<T>;
    /** @internal */ resources(): Set<Resource>;

    /**
     * Transforms the data of the output with the provided func.  The result remains a
     * Output so that dependent resources can be properly tracked.
     *
     * 'func' is not allowed to make resources.
     *
     * 'func' can return other Outputs.  This can be handy if you have a Output<SomeVal>
     * and you want to get a transitive dependency of it.  i.e.
     *
     * ```ts
     * var d1: Output<SomeVal>;
     * var d2 = d1.apply(v => v.x.y.OtherOutput); // getting an output off of 'v'.
     * ```
     *
     * In this example, taking a dependency on d2 means a resource will depend on all the resources
     * of d1.  It will *also* depend on the resources of v.x.y.OtherDep.
     *
     * Importantly, the Resources that d2 feels like it will depend on are the same resources as d1.
     * If you need have multiple Outputs and a single Output is needed that combines both
     * set of resources, then 'pulumi.all' should be used instead.
     *
     * This function will only be called execution of a 'pulumi up' request.  It will not run
     * during 'pulumi preview' (as the values of resources are of course not known then). It is not
     * available for functions that end up executing in the cloud during runtime.  To get the value
     * of the Output during cloud runtime execution, use `get()`.
     */
    apply<U>(func: (t: T) => Promise<U>): Output<U>;
    apply<U>(func: (t: T) => OutputInstance<U>): Output<U>;
    apply<U>(func: (t: T) => U): Output<U>;

    /**
     * Retrieves the underlying value of this dependency.
     *
     * This function is only callable in code that runs in the cloud post-deployment.  At this
     * point all Output values will be known and can be safely retrieved. During pulumi deployment
     * or preview execution this must not be called (and will throw).  This is because doing so
     * would allow Output values to flow into Resources while losing the data that would allow
     * the dependency graph to be changed.
     */
    get(): T;
}

/**
 * Static side of the [Output<T>] type.  Can be used to [create] Outputs as well as test
 * arbitrary values to see if they are [Output]s.
 */
export interface OutputConstructor {
    create<T>(val: Input<T>): Output<Unwrap<T>>;
    create<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;

    isInstance<T>(obj: any): obj is Output<T>;

    /** @internal */ new<T>(
            resources: Set<Resource> | Resource[] | Resource,
            promise: Promise<T>,
            isKnown: Promise<boolean>,
            isSecret: Promise<boolean>,
            allResources: Promise<Set<Resource> | Resource[] | Resource>): Output<T>;
}

/**
 * [Output] helps encode the relationship between Resources in a Pulumi application. Specifically an
 * [Output] holds onto a piece of Data and the Resource it was generated from. An [Output] value can
 * then be provided when constructing new Resources, allowing that new Resource to know both the
 * value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be created, which properly tracks the relationship between resources.
 *
 * An [Output] is used in a Pulumi program differently depending on if the application is executing
 * at 'deployment time' (i.e. when actually running the 'pulumi' executable), or at 'run time' (i.e.
 * a piece of code running in some Cloud).
 *
 * At 'deployment time', the correct way to work with the underlying value is to call
 * [Output.apply(func)].  This allows the value to be accessed and manipulated, while still
 * resulting in an [Output] that is keeping track of [Resource]s appropriately.  At deployment time
 * the underlying value may or may not exist (for example, if a preview is being performed).  In
 * this case, the 'func' callback will not be executed, and calling [.apply] will immediately return
 * an [Output] that points to the [undefined] value.  During a normal [update] though, the 'func'
 * callbacks should always be executed.
 *
 * At 'run time', the correct way to work with the underlying value is to simply call [Output.get]
 * which will be promptly return the entire value.  This will be a simple JavaScript object that can
 * be manipulated as necessary.
 *
 * To ease with using [Output]s at 'deployment time', pulumi will 'lift' simple data properties of
 * an underlying value to the [Output] itself.  For example:
 *
 * ```ts
 *      const o: Output<{ name: string, age: number, orders: Order[] }> = ...;
 *      const name : Output<string> = o.name;
 *      const age  : Output<number> = o.age;
 *      const first: Output<Order>  = o.orders[0];
 * ```
 *
 * Instead of having to write:
 *
 * ```ts
 *      const o: Output<{ name: string, age: number, orders: Order[] }> = ...;
 *      const name : Output<string> = o.apply(v => v.name);
 *      const age  : Output<number> = o.apply(v => v.age);
 *      const first: Output<Order> = o.apply(v => v.orders[0]);
 * ```
 */
export type Output<T> = OutputInstance<T> & Lifted<T>;
// tslint:disable-next-line:variable-name
export const Output: OutputConstructor = <any>OutputImpl;

/**
 * The [Lifted] type allows us to express the operation of taking a type, with potentially deeply
 * nested objects and arrays and to then get a type with the same properties, except whose property
 * types are now [Output]s of the original property type.
 *
 * For example:
 *
 *
 *      `type X = { A: string, B: { c: boolean } }`
 *
 * Then `Lifted<X>` would be equivalent to:
 *
 *      `...    = { A: Output<string>, B: Output<{ C: Output<boolean> }> }`
 *
 * [Lifted] is somewhat the opposite of [Unwrap].  It's primary purpose is to allow an instance of
 * [Output<SomeType>] to provide simple access to the properties of [SomeType] directly on the instance
 * itself (instead of haveing to use [.apply]).
 *
 * This lifting only happens through simple pojo objects and arrays.  Functions, for example, are not
 * lifted.  So you cannot do:
 *
 * ```ts
 *      const o: Output<string> = ...;
 *      const c: Output<number> = o.charCodeAt(0);
 * ```
 *
 * Instead, you still need to write;
 *
 * ```ts
 *      const o: Output<string> = ...;
 *      const c: Output<number> = o.apply(v => v.charCodeAt(0));
 * ```
 */
export type Lifted<T> =
    // Specially handle 'string' since TS doesn't map the 'String.Length' property to it.
    T extends string ? LiftedObject<String, NonFunctionPropertyNames<String>> :
    T extends Array<infer U> ? LiftedArray<U> :
    T extends object ? LiftedObject<T, NonFunctionPropertyNames<T>> :
    // fallback to lifting no properties.  Note that `Lifted` is used in
    //    Output<T> = OutputInstance<T> & Lifted<T>
    // so returning an empty object just means that we're adding nothing to Output<T>.
    // This is needed for cases like `Output<any>`.
    {};

// The set of property names in T that are *not* functions.
type NonFunctionPropertyNames<T> = { [K in keyof T]: T[K] extends Function ? never : K }[keyof T];

// Lift up all the non-function properties.  If it was optional before, keep it optional after.
// If it's require before, keep it required afterwards.
export type LiftedObject<T, K extends keyof T> = {
    [P in K]: T[P] extends OutputInstance<infer T1> ? Output<T1> :
              T[P] extends Promise<infer T2> ? Output<T2> : Output<T[P]>
};

export type LiftedArray<T> = {
    /**
      * Gets the length of the array. This is a number one higher than the highest element defined
      * in an array.
      */
    readonly length: Output<number>;

    readonly [n: number]: Output<T>;
};

/**
 * [concat] takes a sequence of [Inputs], stringifies each, and concatenates all values into one
 * final string.  Individual inputs can be any sort of [Input] value.  i.e. they can be [Promise]s,
 * [Output]s, or just plain JavaScript values.  This can be used like so:
 *
 * ```ts
 *      // 'server' and 'loadBalancer' are both resources that expose [Output] properties.
 *      let val: Output<string> = pulumi.concat("http://", server.hostname, ":", loadBalancer.port);
 * ```
 *
 */
export function concat(...params: Input<any>[]): Output<string> {
    return output(params).apply(array => array.join(""));
}

/**
 * [interpolate] is similar to [concat] but is designed to be used as a tagged template expression.
 * i.e.:
 *
 * ```ts
 *      // 'server' and 'loadBalancer' are both resources that expose [Output] properties.
 *      let val: Output<string> = pulumi.interpolate `http://${server.hostname}:${loadBalancer.port}`
 * ```
 *
 * As with [concat] the 'placeholders' between `${}` can be any Inputs.  i.e. they can be
 * [Promise]s, [Output]s, or just plain JavaScript values.
 */
export function interpolate(literals: TemplateStringsArray, ...placeholders: Input<any>[]): Output<string> {
    return output(placeholders).apply(unwrapped => {
        let result = "";

        // interleave the literals with the placeholders
        for (let i = 0; i < unwrapped.length; i++) {
            result += literals[i];
            result += unwrapped[i];
        }

        // add the last literal
        result += literals[literals.length - 1];
        return result;
    });
}
