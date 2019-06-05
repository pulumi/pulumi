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
import { all, Input, Inputs, interpolate, Output, output } from "./output";
import { readResource, registerResource, registerResourceOutputs } from "./runtime/resource";
import { getProject, getStack } from "./runtime/settings";
import * as utils from "./utils";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * createUrn computes a URN from the combination of a resource name, resource type, optional parent,
 * optional project and optional stack.
 */
export function createUrn(name: Input<string>, type: Input<string>, parent?: Resource | Input<URN>, project?: string, stack?: string): Output<string> {
    let parentPrefix: Output<string>;
    if (parent) {
        let parentUrn: Output<string>;
        if (Resource.isInstance(parent)) {
            parentUrn = parent.urn;
        } else {
            parentUrn = output(parent);
        }
        parentPrefix = parentUrn.apply(parentUrnString => parentUrnString.substring(0, parentUrnString.lastIndexOf("::")) + "$");
    } else {
        parentPrefix = output(`urn:pulumi:${stack || getStack()}::${project || getProject()}::`);
    }
    return interpolate`${parentPrefix}${type}::${name}`;
}

// inheritedChildAlias computes the alias that should be applied to a child based on an alias applied to it's parent.
// This may involve changing the name of the resource in cases where the resource has a named derived from the name of
// the parent, and the parent name changed.
function inheritedChildAlias(childName: string, parentName: string, parentAlias: Input<string>, childType: string): Output<string> {
    // If the child name has the parent name as a prefix, then we make the assumption that it was
    // constructed from the convention of using `{name}-details` as the name of the child resource.  To
    // ensure this is aliased correctly, we must then also replace the parent aliases name in the prefix of
    // the child resource name.
    //
    // For example:
    // * name: "newapp-function"
    // * opts.parent.__name: "newapp"
    // * parentAlias: "urn:pulumi:stackname::projectname::awsx:ec2:Vpc::app"
    // * parentAliasName: "app"
    // * aliasName: "app-function"
    // * childAlias: "urn:pulumi:stackname::projectname::aws:s3/bucket:Bucket::app-function"
    let aliasName = output(childName);
    if (childName.startsWith(parentName)) {
        aliasName = output(parentAlias).apply(parentAliasUrn => {
            const parentAliasName = parentAliasUrn.substring(parentAliasUrn.lastIndexOf("::") + 2);
            return parentAliasName + childName.substring(parentName.length);
        });
    }
    return createUrn(aliasName, childType, parentAlias);
}

/**
 * Resource represents a class whose CRUD operations are implemented by a provider plugin.
 */
export abstract class Resource {
    /**
     * @internal
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiResource: boolean = true;

    /**
     * @internal
     * The optional parent of this resource.
     */
    // tslint:disable-next-line:variable-name
    public readonly __parentResource: Resource | undefined;

    /**
     * @internal
     * The child resources of this resource.  We use these (only from a ComponentResource) to allow
     * code to dependOn a ComponentResource and have that effectively mean that it is depending on
     * all the CustomResource children of that component.
     *
     * Important!  We only walk through ComponentResources.  They're the only resources that serve
     * as an aggregation of other primitive (i.e. custom) resources.  While a custom resource can be
     * a parent of other resources, we don't want to ever depend on those child resource.  If we do,
     * it's simple to end up in a situation where we end up depending on a child resource that has a
     * data cycle dependency due to the data passed into it.
     *
     * An example of how this would be bad is:
     *
     * ```ts
     *     var c1 = new CustomResource("c1");
     *     var c2 = new CustomResource("c2", { parentId: c1.id }, { parent: c1 });
     *     var c3 = new CustomResource("c3", { parentId: c1.id }, { parent: c1 });
     * ```
     *
     * The problem here is that 'c2' has a data dependency on 'c1'.  If it tries to wait on 'c1' it
     * will walk to the children and wait on them.  This will mean it will wait on 'c3'.  But 'c3'
     * will be waiting in the same manner on 'c2', and a cycle forms.
     *
     * This normally does not happen with ComponentResources as they do not have any data flowing
     * into them. The only way you would be able to have a problem is if you had this sort of coding
     * pattern:
     *
     * ```ts
     *     var c1 = new ComponentResource("c1");
     *     var c2 = new CustomResource("c2", { parentId: c1.urn }, { parent: c1 });
     *     var c3 = new CustomResource("c3", { parentId: c1.urn }, { parent: c1 });
     * ```
     *
     * However, this would be pretty nonsensical as there is zero need for a custom resource to ever
     * need to reference the urn of a component resource.  So it's acceptable if that sort of
     * pattern failed in practice.
     */
    // tslint:disable-next-line:variable-name
    public __childResources: Set<Resource> | undefined;

    /**
     * urn is the stable logical URN used to distinctly address a resource, both before and after
     * deployments.
     */
    public readonly urn: Output<URN>;

    /**
     * @internal
     * When set to true, protect ensures this resource cannot be deleted.
     */
    // tslint:disable-next-line:variable-name
    private readonly __protect: boolean;

    /**
     * @internal
     * A list of aliases applied to this resource.
     */
    // tslint:disable-next-line:variable-name
    readonly __aliases: Input<URN>[];

    /**
     * @internal
     * The name assigned to the resource at construction.
     */
    // tslint:disable-next-line:variable-name
    private readonly __name: string;

    /**
     * @internal
     * The set of providers to use for child resources. Keyed by package name (e.g. "aws").
     */
    // tslint:disable-next-line:variable-name
    private readonly __providers: Record<string, ProviderResource>;

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
        if (opts.parent && !Resource.isInstance(opts.parent)) {
            throw new Error(`Resource parent is not a valid Resource: ${opts.parent}`);
        }

        if (!t) {
            throw new ResourceError("Missing resource type argument", opts.parent);
        }
        if (!name) {
            throw new ResourceError("Missing resource name argument (for URN creation)", opts.parent);
        }

        this.__name = name;

        // Make a shallow clone of opts to ensure we don't modify the value passed in.
        opts = Object.assign({}, opts);

        if (opts.provider && (<ComponentResourceOptions>opts).providers) {
            throw new ResourceError("Do not supply both 'provider' and 'providers' options to a ComponentResource.", opts.parent);
        }

        // Check the parent type if one exists and fill in any default options.
        this.__providers = {};
        if (opts.parent) {
            this.__parentResource = opts.parent;
            this.__parentResource.__childResources = this.__parentResource.__childResources || new Set();
            this.__parentResource.__childResources.add(this);

            if (opts.protect === undefined) {
                opts.protect = opts.parent.__protect;
            }

            // Make a copy of the aliases array, and add to it any implicit aliases inherited from its parent
            opts.aliases = [...(opts.aliases || [])];
            for (const parentAlias of opts.parent.__aliases) {
                opts.aliases.push(inheritedChildAlias(name, opts.parent.__name, parentAlias, t));
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
            // Note: we checked above that at most one of opts.provider or opts.providers is set.

            // If opts.provider is set, treat that as if we were given a array of provider with that
            // single value in it.  Otherwise, take the array or map of providers, convert it to a
            // map and combine with any providers we've already set from our parent.
            const providers = opts.provider
                ? convertToProvidersMap([opts.provider])
                : convertToProvidersMap((<ComponentResourceOptions>opts).providers);
            this.__providers = { ...this.__providers, ...providers };
        }

        this.__protect = !!opts.protect;

        // Collapse any `Alias`es down to URNs. We have to wait until this point to do so because we do not know the
        // default `name` and `type` to apply until we are inside the resource constructor.
        this.__aliases = [];
        if (opts.aliases) {
            for (const alias of opts.aliases) {
                this.__aliases.push(collapseAliasToUrn(alias, name, t, opts.parent));
            }
        }

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

function convertToProvidersMap(providers: Record<string, ProviderResource> | ProviderResource[] | undefined) {
    if (!providers) {
        return {};
    }

    if (!Array.isArray(providers)) {
        return providers;
    }

    const result: Record<string, ProviderResource> = {};
    for (const provider of providers) {
        result[provider.getPackage()] = provider;
    }

    return result;
}

(<any>Resource).doNotCapture = true;

/**
 * Alias is a partial description of prior named used for a resource. It can be processed in the context of a resource
 * creation to determine what the full aliased URN would be.
 */
export interface Alias {
    /**
     * The previous name of the resource.  If not provided, the current name of the resource is used.
     */
    name?: Input<string>;
    /**
     * The previous type of the resource.  If not provided, the current type of the resource is used.
     */
    type?: Input<string>;
    /**
     * The previous parent of the resource.  If not provided, the current parent of the resource is used
     * (`opts.parent` if provided, else the implicit stack resource parent).
     */
    parent?: Resource | Input<URN>;
    /**
     * The previous stack of the resource.  If not provided, defaults to `pulumi.getStack()`.
     */
    stack?: Input<string>;
    /**
     * The previous project of the resource. If not provided, defaults to `pulumi.getProject()`.
     */
    project?: Input<string>;
}

// collapseAliasToUrn turns an Alias into a URN given a set of default data
function collapseAliasToUrn(
    alias: Input<Alias | string>, defaultName: string, defaultType: string,
    defaultParent: Resource | undefined): Output<URN> {

    return output(alias).apply(a => {
        if (typeof a === "string") {
            return output(a);
        } else {
            return createUrn(
                output(a.name).apply(n => n || defaultName),
                output(a.type).apply(ty => ty || defaultType),
                a.parent || defaultParent,
                a.project,
                a.stack,
            );
        }
    });
}

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
    /**
     * Ignore changes to any of the specified properties.
     */
    ignoreChanges?: string[];
    /**
     * An optional version, corresponding to the version of the provider plugin that should be used when operating on
     * this resource. This version overrides the version information inferred from the current package and should
     * rarely be used.
     */
    version?: string;
    /**
     * An optional list of aliases to treat this resource as matching.
     */
    aliases?: Input<URN | Alias>[];
    /**
     * An optional provider to use for this resource's CRUD operations. If no provider is supplied,
     * the default provider for the resource's package will be used. The default provider is pulled
     * from the parent's provider bag (see also ComponentResourceOptions.providers).
     *
     * If this is a [ComponentResourceOptions] do not provide both [provider] and [providers]
     */
    provider?: ProviderResource;
}

/**
 * CustomResourceOptions is a bag of optional settings that control a custom resource's behavior.
 */
export interface CustomResourceOptions extends ResourceOptions {
    /**
     * When set to true, deleteBeforeReplace indicates that this resource should be deleted before its replacement
     * is created when replacement is necessary.
     */
    deleteBeforeReplace?: boolean;

    /**
     * The names of outputs for this resource that should be treated as secrets. This augments the list that
     * the resource provider and pulumi engine already determine based on inputs to your resource. It can be used
     * to mark certain ouputs as a secrets on a per resource basis.
     */
    additionalSecretOutputs?: string[];
}

/**
 * ComponentResourceOptions is a bag of optional settings that control a component resource's behavior.
 */
export interface ComponentResourceOptions extends ResourceOptions {
    /**
     * An optional set of providers to use for child resources. Either keyed by package name (e.g.
     * "aws"), or just provided as an array.  In the latter case, the package name will be retrieved
     * from the provider itself.
     *
     * In the case of a single provider, the options can be simplified to just pass along `provider: theProvider`
     *
     * Note: do not provide both [provider] and [providers];
     */
    providers?: Record<string, ProviderResource> | ProviderResource[];
}

/**
 * CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed
 * by performing external operations on some physical entity.  The engine understands how to diff
 * and perform partial updates of them, and these CRUD operations are implemented in a dynamically
 * loaded plugin for the defining package.
 */
export abstract class CustomResource extends Resource {
    /**
     * @internal
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiCustomResource: boolean;

    /**
     * @internal
     * Private field containing the type ID for this object. Useful for implementing `isInstance` on
     * classes that inherit from `CustomResource`.
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiType: string;

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
    constructor(t: string, name: string, props?: Inputs, opts: CustomResourceOptions = {}) {
        if ((<ComponentResourceOptions>opts).providers) {
            throw new ResourceError("Do not supply 'providers' option to a CustomResource. Did you mean 'provider' instead?", opts.parent);
        }

        super(t, name, true, props, opts);
        this.__pulumiCustomResource = true;
        this.__pulumiType = t;
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
    constructor(private readonly pkg: string, name: string, props?: Inputs, opts: ResourceOptions = {}) {
        super(`pulumi:providers:${pkg}`, name, props, opts);
    }

    /** @internal */
    public getPackage() {
        return this.pkg;
    }
}

/**
 * ComponentResource is a resource that aggregates one or more other child resources into a higher
 * level abstraction. The component resource itself is a resource, but does not require custom CRUD
 * operations for provisioning.
 */
export class ComponentResource extends Resource {
    /**
     * @internal
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiComponentResource: boolean;

    /**
     * Returns true if the given object is an instance of CustomResource.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is ComponentResource {
        return utils.isInstance<ComponentResource>(obj, "__pulumiComponentResource");
    }

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
        // Explicitly ignore the props passed in.  We allow them for back compat reasons.  However,
        // we explicitly do not want to pass them along to the engine.  The ComponentResource acts
        // only as a container for other resources.  Another way to think about this is that a normal
        // 'custom resource' corresponds to real piece of cloud infrastructure.  So, when it changes
        // in some way, the cloud resource needs to be updated (and vice versa).  That is not true
        // for a component resource.  The component is just used for organizational purposes and does
        // not correspond to a real piece of cloud infrastructure.  As such, changes to it *itself*
        // do not have any effect on the cloud side of things at all.
        super(type, name, /*custom:*/ false, /*props:*/ {}, opts);
        this.__pulumiComponentResource = true;
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

/** @internal */
export const testingOptions = {
    isDryRun: false,
};
