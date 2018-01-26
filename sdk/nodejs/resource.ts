// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
    public readonly urn: Computed<URN>;

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
    constructor(t: string, name: string, custom: boolean, props: ComputedValues = {}, opts: ResourceOptions = {}) {
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
    public readonly id: Computed<ID>;

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
    constructor(t: string, name: string, props?: ComputedValues, opts?: ResourceOptions) {
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
    constructor(t: string, name: string, props?: ComputedValues, opts?: ResourceOptions) {
        super(t, name, false, props, opts);
    }

    // registerOutputs registers synthetic outputs that a component has initialized, usually by allocating
    // other child sub-resources and propagating their resulting property values.
    protected registerOutputs(outputs: ComputedValues | undefined): void {
        if (outputs) {
            registerResourceOutputs(this, outputs);
        }
    }
}

/**
 * Dependency helps encode the relationship between Resources in a Pulumi application. Specifically
 * a Dependency holds onto a piece of Data and the Resource it was generated from. A Dependency
 * value can then be provided when constructing new Resources, allowing that new Resource to know
 * both the value as well as the Resource the value came from.  This allows for a precise 'Resource
 * dependency graph' to be crated, which properly tracks the relationship between resources.
 */
export class Dependency<T> {
    // Internal implementation details. Hidden from the .d.ts file by using @internal. Users are not
    //  allowed to call these methods.  If they do, pulumi cannot provide any guarantees.  TODO: we
    //  could make this all hidden by using weakmaps and storinthe data in a side table.  But that's
    //  likely not necessary to do.

    // What do show for this Dependency during preview. i.e. something like "table.PropName".
    //
    // Only callable on the outside.  Should only be called during preview.
    /* @internal */ public readonly previewDisplay: string;

    // Method that actually produces the concrete value of this dependency, as well as the total
    // deployment-time set of resources this dependency depends on.  This code path will end up
    // executing apply funcs, and should only be called during real deployment and not during
    // previews.
    //
    // Only callable on the outside.
    /* @internal */ public readonly getValue: () => Promise<T>;

    // The list of resource that this dependency value depends on.
    //
    // Only callable on the outside.
    /* @internal */ public readonly resources: () => Set<Resource>;

    // Transforms the data of the dependency with the provided func.  The result remains a Dependency
    // so that dependent resources can be properly tracked.
    //
    // The inner func should not return a Dependency itself. (TODO: can we check for that?)
    //
    // 'func' is not allowed to make resources.
    //
    // Outside only.  Note: this is the *only* outside public API.
    public readonly apply: <U>(func: (t: T) => U) => Dependency<U>;

    /* @internal */ public constructor(
            display: string,
            resources: Set<Resource>, createComputeValueTask: () => Promise<T>) {

        this.previewDisplay = display;

        // Always create a copy so that no one accidentally modifies our Resource list.
        this.resources = () => new Set<Resource>(resources);

        // getValue is lazy.  i.e. we will only apply funcs when asked the first time, and we will
        // also only apply them once (no matter how many times getValue() is called).

        let computeValueTask: Promise<T> | undefined = undefined;
        this.getValue = () => {
            if (!computeValueTask) {
                computeValueTask = createComputeValueTask();
            }

            return computeValueTask;
        };

        this.apply = <U>(func: (t: T) => U) => {
            // Wrap the display with <> to indicate that it's been transformed in some manner.
            // However, don't bother doing this if we're already wrapping some transformed
            // dependency.  i.e. we'll only ever show 'table.prop' or '<table.prop>', not
            // '<<<<table.prop>>>>'.
            const innerDisplay = display.length > 0 && display.charAt(0) === "<"
                ? display
                : "<" + display + ">";

            return new Dependency<U>(
                display,
                resources,
                () => this.getValue().then(func));
        };
    }

    // Retrieves the underlying value of this dependency.
    //
    // Inside only.  Note: this is the *only* inside API available.
    public get(): T {
        throw new Error("Cannot call during deployment.");
    }
}

// Helper function actually allow Resource to create Dependency objects for its output properties.
// Should only be called by pulumi, not by users (TODO: i think).
export function createDependency<T>(previewDisplay: string, resource: Resource, value: Promise<T>): Dependency<T> {
    return new Dependency<T>(previewDisplay, new Set<Resource>([resource]), () => value);
}

// tslint:disable:max-line-length
export function combine<T1, T2>(d1: Dependency<T1>, d2: Dependency<T2>): Dependency<[T1, T2]>;
export function combine<T1, T2, T3>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>): Dependency<[T1, T2, T3]>;
export function combine<T1, T2, T3, T4>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>, d4: Dependency<T4>): Dependency<[T1, T2, T3, T4]>;
export function combine<T1, T2, T3, T4, T5>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>, d4: Dependency<T4>, d5: Dependency<T5>): Dependency<[T1, T2, T3, T4, T5]>;
export function combine<T1, T2, T3, T4, T5, T6>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>, d4: Dependency<T4>, d5: Dependency<T5>, d6: Dependency<T6>): Dependency<[T1, T2, T3, T4, T5, T6]>;
export function combine<T1, T2, T3, T4, T5, T6, T7>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>, d4: Dependency<T4>, d5: Dependency<T5>, d6: Dependency<T6>, d7: Dependency<T7>): Dependency<[T1, T2, T3, T4, T5, T6, T7]>;
export function combine<T1, T2, T3, T4, T5, T6, T7, T8>(d1: Dependency<T1>, d2: Dependency<T2>, d3: Dependency<T3>, d4: Dependency<T4>, d5: Dependency<T5>, d6: Dependency<T6>, d7: Dependency<T7>, d8: Dependency<T8>): Dependency<[T1, T2, T3, T4, T5, T6, T7, T8]>;
export function combine<T>(...ds: Dependency<T>[]): Dependency<T[]>;
export function combine(...ds: Dependency<{}>[]): Dependency<{}[]> {
    const allResources = new Set<Resource>();
    ds.forEach(d => d.resources().forEach(r => allResources.add(r)));

    const previewDisplay = "(" + ds.map(d => d.previewDisplay).join(", ") + ")";

    return new Dependency<{}[]>(
        previewDisplay,
        allResources,
        () => Promise.all(ds.map(d => d.getValue())));
}

/**
 * Computed is a property output for a resource.  It is just a promise that also permits undefined
 * values.  The undefined values are used during planning, when the actual final value of a resource
 * may not yet be known.
 */
export type Computed<T> = Dependency<T>;

/**
 * ComputedValue is a property input for a resource.  It may be a promptly available T or a promise
 * for one.
 */
export type ComputedValue<T> = T | Dependency<T>;

/**
 * ComputedValues is a map of property name to optional property input, one for each resource
 * property value.
 */
export type ComputedValues = { [key: string]: ComputedValue<any> };
