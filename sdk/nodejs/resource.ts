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

import { RunError } from "./errors";
import * as runtime from "./runtime";
import { readResource, registerResource, registerResourceOutputs } from "./runtime/resource";
import { getRootResource } from "./runtime/settings";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * Resource represents a class whose CRUD operations are implemented by a provider plugin.
 */
export abstract class Resource {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
     // tslint:disable-next-line:variable-name
     /* @internal */ private readonly __pulumiResource: boolean = true;

    /**
     * urn is the stable logical URN used to distinctly address a resource, both before and after
     * deployments.
     */
    public readonly urn: Output<URN>;

    /**
     * When set to true, protect ensures this resource cannot be deleted.
     */
     // tslint:disable-next-line:variable-name
    /* @internal */ private readonly __protect: boolean;

    /**
     * The set of providers to use for child resources. Keyed by package name (e.g. "aws").
     */
     // tslint:disable-next-line:variable-name
    /* @internal */ private readonly __providers: Record<string, ProviderResource>;

    public static isInstance(obj: any): obj is Resource {
        return obj && obj.__pulumiResource;
    }

    // getProvider fetches the provider for the given module member, if any.
    public getProvider(moduleMember: string): ProviderResource | undefined {
        const memComponents = moduleMember.split(":");
        if (memComponents.length !== 3) {
            return undefined;
        }

        const pkg = memComponents[0];
        return this.__providers[pkg];
    }

    /**
     * Creates and registers a new resource object.  t is the fully qualified type token and name is
     * the "name" part to use in creating a stable and globally unique URN for the object.
     * dependsOn is an optional list of other resources that this resource depends on, controlling
     * the order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unique_ name of the resource.
     * @param custom True to indicate that this is a custom resource, managed by a plugin.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(t: string, name: string, custom: boolean, props: Inputs = {}, opts: ResourceOptions = {}) {
        if (!t) {
            throw new RunError("Missing resource type argument");
        }
        if (!name) {
            throw new RunError("Missing resource name argument (for URN creation)");
        }

        // Check the parent type if one exists and fill in any default options.
        this.__providers = {};
        if (opts.parent) {
            if (!Resource.isInstance(opts.parent)) {
                throw new RunError(`Resource parent is not a valid Resource: ${opts.parent}`);
            }

            if (opts.protect === undefined) {
                opts.protect = opts.parent.__protect;
            }

            this.__providers = opts.parent.__providers;

            if (custom) {
                const provider = (<CustomResourceOptions>opts).provider;
                if (provider === undefined) {
                    (<CustomResourceOptions>opts).provider = opts.parent.getProvider(t);
                }
            }
        }
        if (!custom) {
            const providers = (<ComponentResourceOptions>opts).providers;
            if (providers) {
                this.__providers = { ...this.__providers, ...providers };
            }
        }
        this.__protect = !!opts.protect;

        if (opts.id) {
            // If this resource already exists, read its state rather than registering it anew.
            if (!custom) {
                throw new RunError("Cannot read an existing resource unless it has a custom provider");
            }
            readResource(this, t, name, props, opts);
        } else {
            // Kick off the resource registration.  If we are actually performing a deployment, this
            // resource's properties will be resolved asynchronously after the operation completes, so
            // that dependent computations resolve normally.  If we are just planning, on the other
            // hand, values will never resolve.
            registerResource(this, t, name, custom, props, opts);
        }
    }
}

(<any>Resource).doNotCapture = true;

/**
 * ResourceOptions is a bag of optional settings that control a resource's behavior.
 */
export interface ResourceOptions {
    /**
     * An optional existing ID to load, rather than create.
     */
    id?: Input<ID>;
    /**
     * An optional parent resource to which this resource belongs.
     */
    parent?: Resource;
    /**
     * An optional additional explicit dependencies on other resources.
     */
    dependsOn?: Resource[] | Resource;
    /**
     * When set to true, protect ensures this resource cannot be deleted.
     */
    protect?: boolean;
}

/**
 * CustomResourceOptions is a bag of optional settings that control a custom resource's behavior.
 */
export interface CustomResourceOptions extends ResourceOptions {
    /**
     * An optional provider to use for this resource's CRUD operations. If no provider is supplied, the default
     * provider for the resource's package will be used. The default provider is pulled from the parent's
     * provider bag (see also ComponentResourceOptions.providers).
     */
    provider?: ProviderResource;
}

/**
 * ComponentResourceOptions is a bag of optional settings that control a component resource's behavior.
 */
export interface ComponentResourceOptions extends ResourceOptions {
    /**
     * An optional set of providers to use for child resources. Keyed by package name (e.g. "aws")
     */
    providers?: Record<string, ProviderResource>;
}

/**
 * CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed
 * by performing external operations on some physical entity.  The engine understands how to diff
 * and perform partial updates of them, and these CRUD operations are implemented in a dynamically
 * loaded plugin for the defining package.
 */
export abstract class CustomResource extends Resource {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ private readonly __pulumiCustomResource: boolean = true;

    /**
     * id is the provider-assigned unique ID for this managed resource.  It is set during
     * deployments and may be missing (undefined) during planning phases.
     */
    public readonly id: Output<ID>;

    /**
     * Returns true if the given object is an instance of CustomResource.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is CustomResource {
        return obj && obj.__pulumiCustomResource;
    }

    /**
     * Creates and registers a new managed resource.  t is the fully qualified type token and name
     * is the "name" part to use in creating a stable and globally unique URN for the object.
     * dependsOn is an optional list of other resources that this resource depends on, controlling
     * the order in which we perform resource operations. Creating an instance does not necessarily
     * perform a create on the physical entity which it represents, and instead, this is dependent
     * upon the diffing of the new goal state compared to the current known resource state.
     *
     * @param t The type of the resource.
     * @param name The _unique_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(t: string, name: string, props?: Inputs, opts?: CustomResourceOptions) {
        super(t, name, true, props, opts);
    }
}

(<any>CustomResource).doNotCapture = true;

/**
 * ProviderResource is a resource that implements CRUD operations for other custom resources. These resources are
 * managed similarly to other resources, including the usual diffing and update semantics.
 */
export abstract class ProviderResource extends CustomResource {
    /**
     * Creates and registers a new provider resource for a particular package.
     *
     * @param pkg The package associated with this provider.
     * @param name The _unique_ name of the provider.
     * @param props The configuration to use for this provider.
     * @param opts A bag of options that control this provider's behavior.
     */
    constructor(pkg: string, name: string, props?: Inputs, opts?: ResourceOptions) {
        if (opts && (<any>opts).provider !== undefined) {
            throw new RunError("Explicit providers may not be used with provider resources");
        }

        super(`pulumi:providers:${pkg}`, name, props, opts);
    }
}

/**
 * ComponentResource is a resource that aggregates one or more other child resources into a higher
 * level abstraction. The component resource itself is a resource, but does not require custom CRUD
 * operations for provisioning.
 */
export class ComponentResource extends Resource {
    /**
     * Creates and registers a new component resource.  t is the fully qualified type token and name
     * is the "name" part to use in creating a stable and globally unique URN for the object. parent
     * is the optional parent for this component, and dependsOn is an optional list of other
     * resources that this resource depends on, controlling the order in which we perform resource
     * operations.
     *
     * @param t The type of the resource.
     * @param name The _unique_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(t: string, name: string, props?: Inputs, opts?: ComponentResourceOptions) {
        if (opts && (<any>opts).provider !== undefined) {
            throw new RunError("Explicit providers may not be used with component resources");
        }

        super(t, name, false, props, opts);
    }

    // registerOutputs registers synthetic outputs that a component has initialized, usually by allocating
    // other child sub-resources and propagating their resulting property values.
    protected registerOutputs(outputs: Inputs | Promise<Inputs> | Output<Inputs> | undefined): void {
        if (outputs) {
            registerResourceOutputs(this, outputs);
        }
    }
}

(<any>ComponentResource).doNotCapture = true;
(<any>ComponentResource.prototype).registerOutputs.doNotCapture = true;

/**
 * Output helps encode the relationship between Resources in a Pulumi application. Specifically an
 * Output holds onto a piece of Data and the Resource it was generated from. An Output value can
 * then be provided when constructing new Resources, allowing that new Resource to know both the
 * value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be created, which properly tracks the relationship between resources.
 */
export class Output<T> {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * This is internal instead of being truly private, to support mixins and our serialization model.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __pulumiOutput?: boolean = true;

    /**
     * Whether or not this 'Output' should actually perform .apply calls.  During a preview,
     * an Output value may not be known (because it would have to actually be computed by doing an
     * 'update').  In that case, we don't want to perform any .apply calls as the callbacks
     * may not expect an undefined value.  So, instead, we just transition to another Output
     * value that itself knows it should not perform .apply calls.
     */
    /* @internal */ public isKnown: Promise<boolean>;

    /**
     * Method that actually produces the concrete value of this output, as well as the total
     * deployment-time set of resources this output depends on.
     *
     * Only callable on the outside.
     */
    /* @internal */ public readonly promise: () => Promise<T>;

    /**
     * The list of resource that this output value depends on.
     *
     * Only callable on the outside.
     */
    /* @internal */ public readonly resources: () => Set<Resource>;

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
     * of d1.  It will *not* depend on the resources of v.x.y.OtherDep.
     *
     * Importantly, the Resources that d2 feels like it will depend on are the same resources as d1.
     * If you need have multiple Outputs and a single Output is needed that combines both
     * set of resources, then 'pulumi.all' should be used instead.
     *
     * This function will only be called execution of a 'pulumi update' request.  It will not run
     * during 'pulumi preview' (as the values of resources are of course not known then). It is not
     * available for functions that end up executing in the cloud during runtime.  To get the value
     * of the Output during cloud runtime execution, use `get()`.
     */
    public readonly apply: <U>(func: (t: T) => Input<U>) => Output<U>;

    /**
     * Retrieves the underlying value of this dependency.
     *
     * This function is only callable in code that runs in the cloud post-deployment.  At this
     * point all Output values will be known and can be safely retrieved. During pulumi deployment
     * or preview execution this must not be called (and will throw).  This is because doing so
     * would allow Output values to flow into Resources while losing the data that would allow
     * the dependency graph to be changed.
     */
     public readonly get: () => T;

    // Statics

    /**
     * Returns true if the given object is an instance of Output<T>.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance<T>(obj: any): obj is Output<T> {
        return obj && obj.__pulumiOutput;
    }

    /* @internal */ public static create<T>(
            resource: Resource, promise: Promise<T>, isKnown: Promise<boolean>): Output<T> {
        return new Output<T>(new Set<Resource>([resource]), promise, isKnown);
    }

    /* @internal */ public constructor(
            resources: Set<Resource>, promise: Promise<T>, isKnown: Promise<boolean>) {
        this.isKnown = isKnown;

        // Always create a copy so that no one accidentally modifies our Resource list.
        this.resources = () => new Set<Resource>(resources);

        this.promise = () => promise;

        this.apply = <U>(func: (t: T) => Input<U>) => {
            let innerIsKnownResolve: (val: boolean) => void;
            let innerIsKnownReject: (reason: any) => void;
            const innerIsKnown = new Promise<boolean>((resolve, reject) => {
                innerIsKnownResolve = resolve;
                innerIsKnownReject = reject;
            });

            // The known state of the output we're returning depends on if we're known as well, and
            // if a potential lifted inner Output is known.  If we get an inner Output, and it is
            // not known itself, then the result we return should not be known.
            const resultIsKnown = Promise.all([isKnown, innerIsKnown]).then(([k1, k2]) => k1 && k2);

            return new Output<U>(resources, promise.then(async v => {
                try {
                    // During previews do not perform the apply if the engine was not able to
                    // give us an actual value for this Output.
                    const perform = await isKnown;
                    if (runtime.isDryRun() && !perform) {
                        // We couldn't run the function, our new Output is definitely **not** known.
                        innerIsKnownResolve(false);
                        return <U><any>undefined;
                    }

                    const transformed = await func(v);
                    if (Output.isInstance(transformed)) {
                        // Note: if the func returned a Output, we unwrap that to get the inner value
                        // returned by that Output.  Note that we are *not* capturing the Resources of
                        // this inner Output.  That's intentional.  As the Output returned is only
                        // supposed to be related this *this* Output object, those resources should
                        // already be in our transitively reachable resource graph.

                        // The callback func has produced an inner Output that may be 'known' or 'unknown'.
                        // We have to properly forward that along to our outer output.  That way the Outer
                        // output doesn't consider itself 'known' then the inner Output did not.
                        transformed.isKnown.then(innerIsKnownResolve, innerIsKnownReject);
                        return await transformed.promise();
                    } else {
                        // We successfully ran the inner function.  Our new Output should be considered known.
                        innerIsKnownResolve(true);
                        return transformed;
                    }
                } catch (err) {
                    // If anything failed along the way, reject the isKnown bit as well for the outer
                    // Output, so that that exception can propogate as well for anyone awaiting that
                    // promise.
                    innerIsKnownReject(err);
                    throw err;
                }
            }), resultIsKnown);
        };

        this.get = () => {
            throw new RunError(`Cannot call '.get' during update or preview.
To manipulate the value of this Output, use '.apply' instead.`);
        };
    }
}

/**
 * [output] takes any Input value and converts it into an Output, deeply unwrapping nested Input
 * values as necessary".
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
    if (val === null || typeof val !== "object") {
        // strings, numbers, booleans, functions, symbols, undefineds, nulls are all returned as
        // themselves.  They are always 'known' (i.e. we can safely 'apply' off of them even during
        // preview).
        return createSimpleOutput(val);
    }
    else if (Resource.isInstance(val)) {
        // Don't unwrap Resources, there are existing codepaths that return Resources through
        // Outputs and we want to preserve them as is when flattening.
        return createSimpleOutput(val);
    }
    else if (val instanceof Promise) {
        // For a promise, we can just treat the same as an output that points to that resource. So
        // we just create an Output around the Promise, and immediately apply the unwrap function on
        // it to transform the value it points at.
        return <any>new Output(new Set(), val, /*isKnown*/ Promise.resolve(true)).apply(output);
    }
    else if (Output.isInstance(val)) {
        return <any>val.apply(output);
    }
    else if (val instanceof Array) {
        return <any>all(val.map(output));
    }
    else {
        const unwrappedObject: any = {};
        Object.keys(val).forEach(k => {
            unwrappedObject[k] = output((<any>val)[k]);
        });

        return <any>all(unwrappedObject);
    }
}

function createSimpleOutput(val: any) {
    return new Output(new Set(), Promise.resolve(val), /*isKnown*/ Promise.resolve(true));
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
    if (val instanceof Array) {
        const allOutputs = val.map(v => output(v));

        const [resources, isKnown] = getResourcesAndIsKnown(allOutputs);
        const promisedArray = Promise.all(allOutputs.map(o => o.promise()));

        return new Output<Unwrap<T>[]>(new Set<Resource>(resources), promisedArray, isKnown);
    } else {
        const keysAndOutputs = Object.keys(val).map(key => ({ key, value: output(val[key]) }));
        const allOutputs = keysAndOutputs.map(kvp => kvp.value);

        const [resources, isKnown] = getResourcesAndIsKnown(allOutputs);
        const promisedObject = getPromisedObject(keysAndOutputs);

        return new Output<Record<string, Unwrap<T>>>(new Set<Resource>(resources), promisedObject, isKnown);
    }
}

async function getPromisedObject<T>(keysAndOutputs: { key: string, value: Output<Unwrap<T>> }[]): Promise<Record<string, Unwrap<T>>> {
    const result: Record<string, Unwrap<T>> = {};
    for (const kvp of keysAndOutputs) {
        result[kvp.key] = await kvp.value.promise();
    }

    return result;
}

function getResourcesAndIsKnown<T>(allOutputs: Output<Unwrap<T>>[]): [Resource[], Promise<boolean>] {
    const allResources = allOutputs.reduce<Resource[]>((arr, o) => (arr.push(...o.resources()), arr), []);

    // A merged output is known if all of its inputs are known.
    const isKnown = Promise.all(allOutputs.map(o => o.isKnown)).then(ps => ps.every(b => b));

    return [allResources, isKnown];
}

/**
 * Input is a property input for a resource.  It may be a promptly available T, a promise
 * for one, or the output from a existing Resource.
 */
export type Input<T> = T | Promise<T> | Output<T>;

/**
 * Inputs is a map of property name to property input, one for each resource
 * property value.
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
type Unwrap<T> =
    // 1. If we have a promise, just get the type it itself is wrapping and recursively unwrap that.
    // 2. Otherwise, if we have an output, do the same as a promise and just unwrap the inner type.
    // 3. Otherwise, we have a basic type.  Just unwrap that.
    T extends Promise<infer U1> ? UnwrapSimple<U1> :
    T extends Output<infer U2> ? UnwrapSimple<U2> :
    UnwrapSimple<T>;

type primitive = string | number | boolean | undefined | null;

/**
 * Handles encountering basic types when unwrapping.
 */
type UnwrapSimple<T> =
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

interface UnwrappedArray<T> extends Array<Unwrap<T>> {}

type UnwrappedObject<T> = {
    [P in keyof T]: Unwrap<T[P]>;
};
