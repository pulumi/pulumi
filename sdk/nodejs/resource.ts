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

import { ResourceError } from "./errors";
import * as runtime from "./runtime";
import { readResource, registerResource, registerResourceOutputs } from "./runtime/resource";
import * as utils from "./utils";

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
     /* @internal */ public readonly __pulumiResource: boolean = true;

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
        return utils.isInstance<Resource>(obj, "__pulumiResource");
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
     * Creates and registers a new resource object.  [t] is the fully qualified type token and
     * [name] is the "name" part to use in creating a stable and globally unique URN for the object.
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
            throw new ResourceError("Missing resource type argument", opts.parent);
        }
        if (!name) {
            throw new ResourceError("Missing resource name argument (for URN creation)", opts.parent);
        }

        // Check the parent type if one exists and fill in any default options.
        this.__providers = {};
        if (opts.parent) {
            if (!Resource.isInstance(opts.parent)) {
                throw new ResourceError(`Resource parent is not a valid Resource: ${opts.parent}`, opts.parent);
            }

            if (opts.protect === undefined) {
                opts.protect = opts.parent.__protect;
            }

            this.__providers = opts.parent.__providers;

            if (custom) {
                const provider = (<CustomResourceOptions>opts).provider;
                if (provider === undefined) {
                    (<CustomResourceOptions>opts).provider = opts.parent.getProvider(t);
                } else {
                    // If a provider was specified, add it to the providers map under this type's package so that
                    // any children of this resource inherit its provider.
                    const typeComponents = t.split(":");
                    if (typeComponents.length === 3) {
                        const pkg = typeComponents[0];
                        this.__providers = { ...this.__providers, [pkg]: provider };
                    }
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
                throw new ResourceError(
                    "Cannot read an existing resource unless it has a custom provider", opts.parent);
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
    dependsOn?: Input<Input<Resource>[]> | Input<Resource>;
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
    /* @internal */ public readonly __pulumiCustomResource: boolean = true;

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
        return utils.isInstance<CustomResource>(obj, "__pulumiCustomResource");
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
    constructor(pkg: string, name: string, props?: Inputs, opts: ResourceOptions = {}) {
        if ((<any>opts).provider !== undefined) {
            throw new ResourceError("Explicit providers may not be used with provider resources", opts.parent);
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
     * Creates and registers a new component resource.  [type] is the fully qualified type token and
     * [name] is the "name" part to use in creating a stable and globally unique URN for the object.
     * [opts.parent] is the optional parent for this component, and [opts.dependsOn] is an optional
     * list of other resources that this resource depends on, controlling the order in which we
     * perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unique_ name of the resource.
     * @param unused [Deprecated].  Component resources do not communicate or store their properties
     *               with the Pulumi engine.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(type: string, name: string, unused?: Inputs, opts: ComponentResourceOptions = {}) {
        if ((<any>opts).provider !== undefined) {
            throw new ResourceError("Explicit providers may not be used with component resources", opts.parent);
        }

        // Explicitly ignore the props passed in.  We allow them for back compat reasons.  However,
        // we explicitly do not want to pass them along to the engine.  The ComponentResource acts
        // only as a container for other resources.  Another way to think about this is that a normal
        // 'custom resource' corresponds to real piece of cloud infrastructure.  So, when it changes
        // in some way, the cloud resource needs to be updated (and vice versa).  That is not true
        // for a component resource.  The component is just used for organizational purposes and does
        // not correspond to a real piece of cloud infrastructure.  As such, changes to it *itself*
        // do not have any effect on the cloud side of things at all.
        super(type, name, /*custom:*/ false, /*props:*/ {}, opts);
    }

    // registerOutputs registers synthetic outputs that a component has initialized, usually by
    // allocating other child sub-resources and propagating their resulting property values.
    // ComponentResources should always call this at the end of their constructor to indicate that
    // they are done creating child resources.  While not strictly necessary, this helps the
    // experience by ensuring the UI transitions the ComponentResource to the 'complete' state as
    // quickly as possible (instead of waiting until the entire application completes).
    protected registerOutputs(outputs?: Inputs | Promise<Inputs> | Output<Inputs>): void {
        registerResourceOutputs(this, outputs || {});
    }
}

(<any>ComponentResource).doNotCapture = true;
(<any>ComponentResource.prototype).registerOutputs.doNotCapture = true;

/* @internal */
export const testingOptions = {
    isDryRun: false,
};

/**
 * Output helps encode the relationship between Resources in a Pulumi application. Specifically an
 * Output holds onto a piece of Data and the Resource it was generated from. An Output value can
 * then be provided when constructing new Resources, allowing that new Resource to know both the
 * value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be created, which properly tracks the relationship between resources.
 */
class OutputImpl<T> {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * This is internal instead of being truly private, to support mixins and our serialization model.
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __pulumiOutput: boolean = true;

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
     * create takes any Input value and converts it into an Output, deeply unwrapping nested Input
     * values as necessary.
     */
    public static create<T>(val: Input<T>): Output<Unwrap<T>>;
    public static create<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;
    public static create<T>(val: Input<T | undefined>): Output<Unwrap<T | undefined>> {
        return output<T>(<any>val);
    }

    /**
     * Returns true if the given object is an instance of Output<T>.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance<T>(obj: any): obj is Output<T> {
        return utils.isInstance(obj, "__pulumiOutput");
    }

    /* @internal */ public constructor(
            resources: Set<Resource> | Resource[] | Resource, promise: Promise<T>, isKnown: Promise<boolean>) {
        this.isKnown = isKnown;

        // Always create a copy so that no one accidentally modifies our Resource list.
        if (Array.isArray(resources)) {
            this.resources = () => new Set<Resource>(resources);
        } else if (resources instanceof Set) {
            this.resources = () => new Set<Resource>(resources);
        } else {
            this.resources = () => new Set<Resource>([resources]);
        }

        this.promise = () => promise;

        this.apply = <any>(<U>(func: (t: T) => Input<U>) => {
            let innerIsKnownResolve: (val: boolean) => void;
            const innerIsKnown = new Promise<boolean>(resolve => {
                innerIsKnownResolve = resolve;
            });

            // The known state of the output we're returning depends on if we're known as well, and
            // if a potential lifted inner Output is known.  If we get an inner Output, and it is
            // not known itself, then the result we return should not be known.
            const resultIsKnown = Promise.all([isKnown, innerIsKnown]).then(([k1, k2]) => k1 && k2);

            return new Output<U>(resources, promise.then(async v => {
                try {
                    if (runtime.isDryRun()) {
                        // During previews only perform the apply if the engine was able to
                        // give us an actual value for this Output.
                        const applyDuringPreview = await isKnown;

                        if (!applyDuringPreview) {
                            // We didn't actually run the function, our new Output is definitely
                            // **not** known.
                            innerIsKnownResolve(false);
                            return <U><any>undefined;
                        }
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
                        innerIsKnownResolve(await transformed.isKnown);
                        return await transformed.promise();
                    } else {
                        // We successfully ran the inner function.  Our new Output should be considered known.
                        innerIsKnownResolve(true);
                        return transformed;
                    }
                }
                finally {
                    // Ensure we always resolve the inner isKnown value no matter what happens
                    // above. If anything failed along the way, consider this output to be
                    // not-known. Awaiting this Output's promise() will still throw, but await'ing
                    // the isKnown bit will just return 'false'.
                    innerIsKnownResolve(false);
                }
            }), resultIsKnown);
        });

        this.get = () => {
            throw new Error(`Cannot call '.get' during update or preview.
To manipulate the value of this Output, use '.apply' instead.`);
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
                return obj.apply(ob => {
                    if (ob === undefined || ob === null) {
                        return undefined;
                    }

                    return ob[prop];
                });
            },
        });
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

async function getPromisedObject<T>(
        keysAndOutputs: { key: string, value: Output<Unwrap<T>> }[]): Promise<Record<string, Unwrap<T>>> {
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
 * [Input] is a property input for a resource.  It may be a promptly available T, a promise for one,
 * or the output from a existing Resource.
 */
export type Input<T> = T | Promise<T> | Output<T>;

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
    T extends Output<infer U2> ? UnwrapSimple<U2> :
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
 * Static side of the [Output<T>] type.  Can be used to [create] Outputs as well as test
 * arbitrary values to see if they are [Output]s.
 */
export interface OutputConstructor {
    create<T>(val: Input<T>): Output<Unwrap<T>>;
    create<T>(val: Input<T> | undefined): Output<Unwrap<T | undefined>>;

    isInstance<T>(obj: any): obj is Output<T>;

    /* @internal */ new<T>(
            resources: Set<Resource> | Resource[] | Resource,
            promise: Promise<T>,
            isKnown: Promise<boolean>): Output<T>;
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
export type Output<T> = OutputImpl<T> & Lifted<T>;
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
    // Output<T> is an intersection type with 'Lifted<T>'.  So, when we don't want to add any
    // members to Output<T>, we just return `{}` which will leave it untouched.
    T extends Function ? {} :
    T extends primitive ? {} :
    T extends Resource ? {} :
    T extends Array<infer U> ? LiftedArray<U> :
    T extends object ? LiftedObject<T> :
    never;

export interface LiftedArray<T> extends Array<Output<T>> {}

export type LiftedObject<T> = {
    [P in keyof T]: Output<T[P]>;
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
