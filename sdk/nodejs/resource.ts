// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as runtime from "./runtime";
import { registerResource, registerResourceOutputs } from "./runtime/resource";
import { getRootResource } from "./runtime/settings";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * Resource represents a class whose CRUD operations are implemented by a provider plugin.
 */
export abstract class Resource {
    /**
     * urn is the stable logical URN used to distinctly address a resource, both before and after
     * deployments.
     */
    public readonly urn: Output<URN>;

    /**
     * Creates and registers a new resource object.  t is the fully qualified type token and name is
     * the "name" part to use in creating a stable and globally unique URN for the object.
     * dependsOn is an optional list of other resources that this resource depends on, controlling
     * the order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param custom True to indicate that this is a custom resource, managed by a plugin.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(t: string, name: string, custom: boolean, props: Inputs = {}, opts: ResourceOptions = {}) {
        if (!t) {
            throw new Error("Missing resource type argument");
        }
        if (!name) {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // If there wasn't an explicit parent, and a root resource exists, parent to that.
        if (!opts.parent) {
            opts.parent = getRootResource();
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this
        // resource's properties will be resolved asynchronously after the operation completes, so
        // that dependent computations resolve normally.  If we are just planning, on the other
        // hand, values will never resolve.
        registerResource(this, t, name, custom, props, opts);
    }
}

/**
 * ResourceOptions is a bag of optional settings that control a resource's behavior.
 */
export interface ResourceOptions {
    /**
     * An optional parent resource to which this resource belongs.
     */
    parent?: Resource;
    /**
     * An optional additional explicit dependencies on other resources.
     */
    dependsOn?: Resource[];
    /**
     * When set to true, protect ensures this resource cannot be deleted.
     */
    protect?: boolean;
}

/**
 * CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed
 * by performing external operations on some physical entity.  The engine understands how to diff
 * and perform partial updates of them, and these CRUD operations are implemented in a dynamically
 * loaded plugin for the defining package.
 */
export abstract class CustomResource extends Resource {
    /**
     * id is the provider-assigned unique ID for this managed resource.  It is set during
     * deployments and may be missing (undefined) during planning phases.
     */
    public readonly id: Output<ID>;

    /**
     * Creates and registers a new managed resource.  t is the fully qualified type token and name
     * is the "name" part to use in creating a stable and globally unique URN for the object.
     * dependsOn is an optional list of other resources that this resource depends on, controlling
     * the order in which we perform resource operations. Creating an instance does not necessarily
     * perform a create on the physical entity which it represents, and instead, this is dependent
     * upon the diffing of the new goal state compared to the current known resource state.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(t: string, name: string, props?: Inputs, opts?: ResourceOptions) {
        super(t, name, true, props, opts);
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
     * @param name The _unqiue_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param opts A bag of options that control this resource's behavior.
     * @param protect True to ensure this resource cannot be deleted.
     */
    constructor(t: string, name: string, props?: Inputs, opts?: ResourceOptions) {
        super(t, name, false, props, opts);
    }

    // registerOutputs registers synthetic outputs that a component has initialized, usually by allocating
    // other child sub-resources and propagating their resulting property values.
    protected registerOutputs(outputs: Inputs | undefined): void {
        if (outputs) {
            registerResourceOutputs(this, outputs);
        }
    }
}

/**
 * Output helps encode the relationship between Resources in a Pulumi application. Specifically
 * an Output holds onto a piece of Data and the Resource it was generated from. An Output
 * value can then be provided when constructing new Resources, allowing that new Resource to know
 * both the value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be crated, which properly tracks the relationship between resources.
 */
export class Output<T> {
    // Method that actually produces the concrete value of this output, as well as the total
    // deployment-time set of resources this output depends on.
    //
    // Only callable on the outside.
    /* @internal */ public readonly promise: () => Promise<T>;

    // The list of resource that this output value depends on.
    //
    // Only callable on the outside.
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
     * of the Output during cloud runtime executure, use `get()`.
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
    /* @internal */ public static create<T>(resource: Resource, promise: Promise<T>): Output<T> {
        return new Output<T>(new Set<Resource>([resource]), promise);
    }

    /* @internal */ public constructor(resources: Set<Resource>, promise: Promise<T>) {
        // Always create a copy so that no one accidentally modifies our Resource list.
        this.resources = () => new Set<Resource>(resources);

        this.promise = () => promise;

        this.apply = <U>(func: (t: T) => Input<U>) => {
            if (runtime.options.dryRun) {
                // During previews we never actually apply the func.
                return new Output<U>(resources, Promise.resolve(<U><any>undefined));
            }

            return new Output<U>(resources, promise.then(async v => {
                const transformed = await func(v);
                if (transformed instanceof Output) {
                    // Note: if the func returned a Output, we unwrap that to get the inner value
                    // returned by that Output.  Note that we are *not* capturing the Resources of
                    // this inner Output.  That's intentional.  As the Output returned is only
                    // supposed to be related this *this* Output object, those resources should
                    // already be in our transitively reachable resource graph.
                    return await transformed.promise();
                } else {
                    return transformed;
                }
            }));
        };


        this.get = () => {
            throw new Error(`Cannot call during deployment or preview.
To manipulate the value of this dependency, use 'apply' instead.`);
        };
    }
}

export function output<T>(cv: Input<T>): Output<T>;
export function output<T>(cv: Input<T> | undefined): Output<T | undefined>;
export function output<T>(cv: Input<T | undefined>): Output<T | undefined> {
    return cv instanceof Output
        ? cv
        : new Output<T | undefined>(new Set<Resource>(), Promise.resolve(cv));
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
 *
 */
// tslint:disable:max-line-length
export function all<T>(val: { [key: string]: Input<T> }): Output<{ [key: string]: T }>;
export function all<T1, T2, T3, T4, T5, T6, T7, T8>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined, Input<T7> | undefined, Input<T8> | undefined]): Output<[T1, T2, T3, T4, T5, T6, T7, T8]>;
export function all<T1, T2, T3, T4, T5, T6, T7>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined, Input<T7> | undefined]): Output<[T1, T2, T3, T4, T5, T6, T7]>;
export function all<T1, T2, T3, T4, T5, T6>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined, Input<T6> | undefined]): Output<[T1, T2, T3, T4, T5, T6]>;
export function all<T1, T2, T3, T4, T5>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined, Input<T5> | undefined]): Output<[T1, T2, T3, T4, T5]>;
export function all<T1, T2, T3, T4>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined, Input<T4> | undefined]): Output<[T1, T2, T3, T4]>;
export function all<T1, T2, T3>(values: [Input<T1> | undefined, Input<T2> | undefined, Input<T3> | undefined]): Output<[T1, T2, T3]>;
export function all<T1, T2>(values: [Input<T1> | undefined, Input<T2> | undefined]): Output<[T1, T2]>;
export function all<T>(ds: (Input<T> | undefined)[]): Output<T[]>;
export function all<T>(val: Input<T>[] | { [key: string]: Input<T> }): Output<any> {
    if (val instanceof Array) {
        const allDeps = val.map(output);

        const resources = allDeps.reduce<Resource[]>((arr, dep) => (arr.push(...dep.resources()), arr), []);
        const promises = allDeps.map(d => d.promise());

        return new Output<T[]>(new Set<Resource>(resources), Promise.all(promises));
    } else {
        const array = Object.keys(val).map(k =>
            output<T>(val[k]).apply(v => ({ key: k, value: v})));

        return all(array).apply(keysAndValues => {
            const result: { [key: string]: T } = {};
            for (const kvp of keysAndValues) {
                result[kvp.key] = kvp.value;
            }

            return result;
        });
    }
}

/**
 * Input is a property input for a resource.  It may be a promptly available T or a promise
 * for one.
 */
export type Input<T> = T | Promise<T> | Output<T>;

/**
 * Inputs is a map of property name to optional property input, one for each resource
 * property value.
 */
export type Inputs = Record<string, Input<any>>;
