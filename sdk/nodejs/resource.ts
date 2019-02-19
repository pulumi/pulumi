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

import { ResourceError, RunError } from "./errors";
import { all, Input, Inputs, Output } from "./output";
import { readResource, registerResource, registerResourceOutputs } from "./runtime/resource";

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
     * The URN to use for this resource when it is being passed in as the parent of another
     * resource.  This will have the same [URN] *data* as [urn].  However, the [Resources] that each
     * points at will be different.  Specifically [__directUrn] will only contain *this* instance
     * as a resource, whereas [urn] will contain *this* instance and all child resources.
     *
     * The reason for both of these fields is so that we have a way for others to depend on a
     * Resource as a logical aggregation of Resources.  i.e. if a [ComponentResource] ends up making
     * a [Role] (and [RolePolicyAttachment]s), then logically, someone depending on that
     * [ComponentResource] should depend on those child resources as well.  [urn] serves this
     * purpose and is the publicly available manner for all resources to take such a dependency.
     *
     * However, when a parent resource actually is *creating* its children, we want to avoid a
     * potential circularity.  i.e. if a parent creates a child, and the child then waits for the
     * parent urn to be complete, then the child may end up waiting on itself *if* the child
     * resource was added into the resources of [urn].  This can happen due to all the async work
     * that happens when creating a resource.
     *
     * In general, the pulumi engine itself should only await on the promise of this property.
     * Awaiting the promise for [urn] could cause deadlocks.  This is not an issue for 
     */
    // tslint:disable-next-line:variable-name
    /* @internal */ public readonly __directUrn: Output<URN>;

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

        // If we have a parent, we want to consider any dependency on it to be a dependency on us as
        // well. i.e. say there is ComponentResource 'r1' which makes CustomResource 'c1' in its
        // construct, and is used as the parent of a CustomResource 'c2' made later on.  If someone
        // other resource depends on 'r1', we want them to actually be dependent 'c1' and 'c2'
        // automatically.  This is important so that the ComponentResource actually represents an
        // aggregation of other resources and that dependsOn meaningfully will block forward
        // movement on the children of the Component actually being created.
        //
        // To do this, we wait until after readResource/registerResource has kicked off.  At this
        // point *this* resource will have promise kicked off to figure out our URN.  So, we then
        // take our parent and update it's URN such that it is only complete once both it's original
        // computation and our URN computation is actually done.
        if (opts.parent) {
            // Stash away the original urn before we update the publicly visible one.
            this.__directUrn = this.urn;

            const finalUrn = all([opts.parent.urn, this.urn]).apply(([u1, _]) => u1);

            // urn is declared as readonly (so that others are not allowed to update it).  So cast
            // to any so we can do it.  We're the only code path that is allowed to change this.
            //
            // Note: this does mean the urn for our parent can go from a 'done' state to an 'undone'
            // state.  That's an acceptable part of our programming model.  If someone waits on 'r1'
            // right after it is created, they will be effectively waiting on 'r1' and 'c1', but not
            // necessarily 'c2'.  Once 'c2' is created, waiting on 'r1' will then wait on all three
            // resources.
            //
            // We feel this is actually a sensible programming model.  When someone waits on a
            // resource they're waiting on what that resource logically represents at that point in
            // their program.  If, later on, that resource logically represents a larger tree (which
            // would only happen if the user program did something to cause that), then waiting should
            // now wait on that new tree.  This model works well for complex ComponentResources that
            // may have their entire tree dynamically created over many steps as a pulumi app runs.
            (<any>opts.parent).urn = finalUrn;
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

    /**
     * When set to true, deleteBeforeReplace indicates that this resource should be deleted before its replacement
     * is created when replacement is necessary.
     */
    deleteBeforeReplace?: boolean;
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
