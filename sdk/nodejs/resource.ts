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
import { Input, Inputs, interpolate, Output, output } from "./output";
import { getStackResource, unknownValue } from "./runtime";
import { getResource, readResource, registerResource, registerResourceOutputs } from "./runtime/resource";
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
     * A private field to help with RTTI that works in SxS scenarios.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiResource: boolean = true;

    /**
     * The optional parent of this resource.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __parentResource: Resource | undefined;

    /**
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
     *
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public __childResources: Set<Resource> | undefined;

    /**
     * urn is the stable logical URN used to distinctly address a resource, both before and after
     * deployments.
     */
    public readonly urn!: Output<URN>;

    /**
     * When set to true, protect ensures this resource cannot be deleted.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    private readonly __protect: boolean;

    /**
     * A collection of transformations to apply as part of resource registration.
     *
     * Note: This is marked optional only because older versions of this library may not have had
     * this property, and marking optional forces consumers of the property to defensively handle
     * cases where they are passed "old" resources.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    __transformations?: ResourceTransformation[];

    /**
     * A list of aliases applied to this resource.
     *
     * Note: This is marked optional only because older versions of this library may not have had
     * this property, and marking optional forces consumers of the property to defensively handle
     * cases where they are passed "old" resources.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    readonly __aliases?: Input<URN>[];

    /**
     * The name assigned to the resource at construction.
     *
     * Note: This is marked optional only because older versions of this library may not have had
     * this property, and marking optional forces consumers of the property to defensively handle
     * cases where they are passed "old" resources.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    private readonly __name?: string;

    /**
     * The set of providers to use for child resources. Keyed by package name (e.g. "aws").
     * @internal
     */
    // tslint:disable-next-line:variable-name
    private readonly __providers: Record<string, ProviderResource>;

    /**
     * The specified provider or provider determined from the parent for custom resources.
     * @internal
     */
    // Note: This is deliberately not named `__provider` as that conflicts with the property
    // used by the `dynamic.Resource` class.
    // tslint:disable-next-line:variable-name
    readonly __prov?: ProviderResource;

    /**
     * The specified provider version.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    readonly __version?: string;

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
     * @param remote True if this is a remote component resource.
     * @param dependency True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(t: string, name: string, custom: boolean, props: Inputs = {}, opts: ResourceOptions = {},
                remote: boolean = false, dependency: boolean = false) {

        if (dependency) {
            this.__protect = false;
            this.__providers = {};
            return;
        }

        if (opts.parent && !Resource.isInstance(opts.parent)) {
            throw new Error(`Resource parent is not a valid Resource: ${opts.parent}`);
        }

        if (!t) {
            throw new ResourceError("Missing resource type argument", opts.parent);
        }
        if (!name) {
            throw new ResourceError("Missing resource name argument (for URN creation)", opts.parent);
        }

        // Before anything else - if there are transformations registered, invoke them in order to transform the properties and
        // options assigned to this resource.
        const parent = opts.parent || getStackResource() || { __transformations: undefined };
        this.__transformations = [ ...(opts.transformations || []), ...(parent.__transformations || []) ];
        for (const transformation of this.__transformations) {
            const tres = transformation({ resource: this, type: t, name, props, opts });
            if (tres) {
                if (tres.opts.parent !== opts.parent) {
                    // This is currently not allowed because the parent tree is needed to establish what
                    // transformation to apply in the first place, and to compute inheritance of other
                    // resource options in the Resource constructor before transformations are run (so
                    // modifying it here would only even partially take affect).  It's theoretically
                    // possible this restriction could be lifted in the future, but for now just
                    // disallow re-parenting resources in transformations to be safe.
                    throw new Error("Transformations cannot currently be used to change the `parent` of a resource.");
                }
                props = tres.props;
                opts = tres.opts;
            }
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
            if (opts.parent.__name) {
                for (const parentAlias of (opts.parent.__aliases || [])) {
                    opts.aliases.push(inheritedChildAlias(name, opts.parent.__name, parentAlias, t));
                }
            }

            this.__providers = opts.parent.__providers;
        }

        if (custom) {
            const provider = opts.provider;
            if (provider === undefined) {
                if (opts.parent) {
                    // If no provider was given, but we have a parent, then inherit the
                    // provider from our parent.
                    opts.provider = opts.parent.getProvider(t);
                }
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
        else {
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
        this.__prov = custom ? opts.provider : undefined;
        this.__version = opts.version;

        // Collapse any `Alias`es down to URNs. We have to wait until this point to do so because we do not know the
        // default `name` and `type` to apply until we are inside the resource constructor.
        this.__aliases = [];
        if (opts.aliases) {
            for (const alias of opts.aliases) {
                this.__aliases.push(collapseAliasToUrn(alias, name, t, opts.parent));
            }
        }

        if (opts.urn) {
            // This is a resource that already exists. Read its state from the engine.
            getResource(this, props, custom, opts.urn);
        }
        else if (opts.id) {
            // If this is a custom resource that already exists, read its state from the provider.
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
            registerResource(this, t, name, custom, remote, urn => new DependencyResource(urn), props, opts);
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
 * Constant to represent the 'root stack' resource for a Pulumi application.  The purpose of this is
 * solely to make it easy to write an [Alias] like so:
 *
 * `aliases: [{ parent: rootStackResource }]`.
 *
 * This indicates that the prior name for a resource was created based on it being parented directly
 * by the stack itself and no other resources.  Note: this is equivalent to:
 *
 * `aliases: [{ parent: undefined }]`
 *
 * However, the former form is preferable as it is more self-descriptive, while the latter may look
 * a bit confusing and may incorrectly look like something that could be removed without changing
 * semantics.
 */
export const rootStackResource: Resource = undefined!;

/**
 * Alias is a partial description of prior named used for a resource. It can be processed in the
 * context of a resource creation to determine what the full aliased URN would be.
 *
 * Note there is a semantic difference between properties being absent from this type and properties
 * having the `undefined` value. Specifically, there is a difference between:
 *
 * ```ts
 * { name: "foo", parent: undefined } // and
 * { name: "foo" }
 * ```
 *
 * The presence of a property indicates if its value should be used.  If absent, then the value is
 * not used.  So, in the above while `alias.parent` is `undefined` for both, the first alias means
 * "the original urn had no parent" while the second alias means "use the current parent".
 *
 * Note: to indicate that a resource was previously parented by the root stack, it is recommended
 * that you use:
 *
 * `aliases: [{ parent: pulumi.rootStackResource }]`
 *
 * This form is self-descriptive and makes the intent clearer than using:
 *
 * `aliases: [{ parent: undefined }]`
 */
export interface Alias {
    /**
     * The previous name of the resource.  If not provided, the current name of the resource is
     * used.
     */
    name?: Input<string>;
    /**
     * The previous type of the resource.  If not provided, the current type of the resource is used.
     */
    type?: Input<string>;

    /**
     * The previous parent of the resource.  If not provided (i.e. `{ name: "foo" }`), the current
     * parent of the resource is used (`opts.parent` if provided, else the implicit stack resource
     * parent).
     *
     * To specify no original parent, use `{ parent: pulumi.rootStackResource }`.
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
        alias: Input<Alias | string>,
        defaultName: string,
        defaultType: string,
        defaultParent: Resource | undefined): Output<URN> {

    return output(alias).apply(a => {
        if (typeof a === "string") {
            return output(a);
        }

        const name = a.hasOwnProperty("name") ? a.name : defaultName;
        const type = a.hasOwnProperty("type") ? a.type : defaultType;
        const parent = a.hasOwnProperty("parent") ? a.parent : defaultParent;
        const project = a.hasOwnProperty("project") ? a.project : getProject();
        const stack = a.hasOwnProperty("stack") ? a.stack : getStack();

        if (name === undefined) {
            throw new Error("No valid 'name' passed in for alias.");
        }

        if (type === undefined) {
            throw new Error("No valid 'type' passed in for alias.");
        }

        return createUrn(name, type, parent, project, stack);
    });
}

/**
 * ResourceOptions is a bag of optional settings that control a resource's behavior.
 */
export interface ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.

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
     * Changes to any of these property paths will force a replacement.  If this list includes `"*"`, changes to any
     * properties will force a replacement.  Initialization errors from previous deployments will require replacement
     * instead of update only if `"*"` is passed.
     */
    replaceOnChanges?: string[];
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
    /**
     * An optional customTimeouts configuration block.
     */
    customTimeouts?: CustomTimeouts;
    /**
     * Optional list of transformations to apply to this resource during construction. The
     * transformations are applied in order, and are applied prior to transformation applied to
     * parents walking from the resource up to the stack.
     */
    transformations?: ResourceTransformation[];
    /**
     * The URN of a previously-registered resource of this type to read from the engine.
     */
    urn?: URN;

    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.
}

export interface CustomTimeouts {
    /**
     * The optional create timeout represented as a string e.g. 5m, 40s, 1d.
     */
    create?: string;
    /**
     * The optional update timeout represented as a string e.g. 5m, 40s, 1d.
     */
    update?: string;
    /**
     * The optional delete timeout represented as a string e.g. 5m, 40s, 1d.
     */
    delete?: string;
}

/**
 * ResourceTransformation is the callback signature for the `transformations` resource option.  A
 * transformation is passed the same set of inputs provided to the `Resource` constructor, and can
 * optionally return back alternate values for the `props` and/or `opts` prior to the resource
 * actually being created.  The effect will be as though those props and opts were passed in place
 * of the original call to the `Resource` constructor.  If the transformation returns undefined,
 * this indicates that the resource will not be transformed.
 */
export type ResourceTransformation = (args: ResourceTransformationArgs) => ResourceTransformationResult | undefined;

/**
 * ResourceTransformationArgs is the argument bag passed to a resource transformation.
 */
export interface ResourceTransformationArgs {
    /**
     * The Resource instance that is being transformed.
     */
    resource: Resource;
    /**
     * The type of the Resource.
     */
    type: string;
    /**
     * The name of the Resource.
     */
    name: string;
    /**
     * The original properties passed to the Resource constructor.
     */
    props: Inputs;
    /**
     * The original resource options passed to the Resource constructor.
     */
    opts: ResourceOptions;
}

/**
 * ResourceTransformationResult is the result that must be returned by a resource transformation
 * callback.  It includes new values to use for the `props` and `opts` of the `Resource` in place of
 * the originally provided values.
 */
export interface ResourceTransformationResult {
    /**
     * The new properties to use in place of the original `props`
     */
    props: Inputs;
    /**
     * The new resource options to use in place of the original `opts`
     */
    opts: ResourceOptions;
}

/**
 * CustomResourceOptions is a bag of optional settings that control a custom resource's behavior.
 */
export interface CustomResourceOptions extends ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.

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

    /**
     * When provided with a resource ID, import indicates that this resource's provider should import its state from
     * the cloud resource with the given ID. The inputs to the resource's constructor must align with the resource's
     * current state. Once a resource has been imported, the import property must be removed from the resource's
     * options.
     */
    import?: ID;

    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.
}

/**
 * ComponentResourceOptions is a bag of optional settings that control a component resource's behavior.
 */
export interface ComponentResourceOptions extends ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.

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

    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.
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
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiCustomResource: boolean;

    /**
     * Private field containing the type ID for this object. Useful for implementing `isInstance` on
     * classes that inherit from `CustomResource`.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiType: string;

    /**
     * id is the provider-assigned unique ID for this managed resource.  It is set during
     * deployments and may be missing (undefined) during planning phases.
     */
    public readonly id!: Output<ID>;

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
     * @param dependency True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(t: string, name: string, props?: Inputs, opts: CustomResourceOptions = {}, dependency = false) {
        if ((<ComponentResourceOptions>opts).providers) {
            throw new ResourceError("Do not supply 'providers' option to a CustomResource. Did you mean 'provider' instead?", opts.parent);
        }

        super(t, name, true, props, opts, false, dependency);
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
    /** @internal */
    private readonly pkg: string;

    /** @internal */
    // tslint:disable-next-line: variable-name
    public __registrationId?: string;

    public static async register(provider: ProviderResource | undefined): Promise<string | undefined> {
        if (provider === undefined) {
            return undefined;
        }

        if (!provider.__registrationId) {
            const providerURN = await provider.urn.promise();
            const providerID = await provider.id.promise() || unknownValue;
            provider.__registrationId = `${providerURN}::${providerID}`;
        }

        return provider.__registrationId;
    }

    /**
     * Creates and registers a new provider resource for a particular package.
     *
     * @param pkg The package associated with this provider.
     * @param name The _unique_ name of the provider.
     * @param props The configuration to use for this provider.
     * @param opts A bag of options that control this provider's behavior.
     * @param dependency True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(pkg: string, name: string, props?: Inputs, opts: ResourceOptions = {}, dependency: boolean = false) {
        super(`pulumi:providers:${pkg}`, name, props, opts, dependency);
        this.pkg = pkg;
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
export class ComponentResource<TData = any> extends Resource {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiComponentResource = true;

    /** @internal */
    // tslint:disable-next-line:variable-name
    public readonly __data: Promise<TData>;

    /** @internal */
    // tslint:disable-next-line:variable-name
    private __registered = false;

    /** @internal */
    // tslint:disable-next-line:variable-name
    public readonly __remote: boolean;

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
     * @param args Information passed to [initialize] method.
     * @param opts A bag of options that control this resource's behavior.
     * @param remote True if this is a remote component resource.
     */
    constructor(type: string, name: string, args: Inputs = {}, opts: ComponentResourceOptions = {}, remote: boolean = false) {
        // Explicitly ignore the props passed in.  We allow them for back compat reasons.  However,
        // we explicitly do not want to pass them along to the engine.  The ComponentResource acts
        // only as a container for other resources.  Another way to think about this is that a normal
        // 'custom resource' corresponds to real piece of cloud infrastructure.  So, when it changes
        // in some way, the cloud resource needs to be updated (and vice versa).  That is not true
        // for a component resource.  The component is just used for organizational purposes and does
        // not correspond to a real piece of cloud infrastructure.  As such, changes to it *itself*
        // do not have any effect on the cloud side of things at all.
        super(type, name, /*custom:*/ false, /*props:*/ remote || opts?.urn ? args : {}, opts, remote);
        this.__remote = remote;
        this.__registered = remote || !!opts?.urn;
        this.__data = remote || opts?.urn ? Promise.resolve(<TData>{}) : this.initializeAndRegisterOutputs(args);
    }

    /** @internal */
    private async initializeAndRegisterOutputs(args: Inputs) {
        const data = await this.initialize(args);
        this.registerOutputs();
        return data;
    }

    /**
     * Can be overridden by a subclass to asynchronously initialize data for this Component
     * automatically when constructed.  The data will be available immediately for subclass
     * constructors to use.  To access the data use `.getData`.
     */
    protected async initialize(args: Inputs): Promise<TData> {
        return <TData>undefined!;
    }

    /**
     * Retrieves the data produces by [initialize].  The data is immediately available in a
     * derived class's constructor after the `super(...)` call to `ComponentResource`.
     */
    protected getData(): Promise<TData> {
        return this.__data;
    }

    /**
     * registerOutputs registers synthetic outputs that a component has initialized, usually by
     * allocating other child sub-resources and propagating their resulting property values.
     *
     * ComponentResources can call this at the end of their constructor to indicate that they are
     * done creating child resources.  This is not strictly necessary as this will automatically be
     * called after the `initialize` method completes.
     */
    protected registerOutputs(outputs?: Inputs | Promise<Inputs> | Output<Inputs>): void {
        if (this.__registered) {
            return;
        }

        this.__registered = true;
        registerResourceOutputs(this, outputs || {});
    }
}

(<any>ComponentResource).doNotCapture = true;
(<any>ComponentResource.prototype).registerOutputs.doNotCapture = true;
(<any>ComponentResource.prototype).initialize.doNotCapture = true;
(<any>ComponentResource.prototype).initializeAndRegisterOutputs.doNotCapture = true;

/** @internal */
export const testingOptions = {
    isDryRun: false,
};

/**
 * [mergeOptions] takes two ResourceOptions values and produces a new ResourceOptions with the
 * respective properties of `opts2` merged over the same properties in `opts1`.  The original
 * options objects will be unchanged.
 *
 * Conceptually property merging follows these basic rules:
 *  1. if the property is a collection, the final value will be a collection containing the values
 *     from each options object.
 *  2. Simple scaler values from `opts2` (i.e. strings, numbers, bools) will replace the values of
 *     `opts1`.
 *  3. `opts2` can have properties explicitly provided with `null` or `undefined` as the value. If
 *     explicitly provided, then that will be the final value in the result.
 *  4. For the purposes of merging `dependsOn`, `provider` and `providers` are always treated as
 *     collections, even if only a single value was provided.
 */
export function mergeOptions(opts1: CustomResourceOptions | undefined, opts2: CustomResourceOptions | undefined): CustomResourceOptions;
export function mergeOptions(opts1: ComponentResourceOptions | undefined, opts2: ComponentResourceOptions | undefined): ComponentResourceOptions;
export function mergeOptions(opts1: ResourceOptions | undefined, opts2: ResourceOptions | undefined): ResourceOptions;
export function mergeOptions(opts1: ResourceOptions | undefined, opts2: ResourceOptions | undefined): ResourceOptions {
    const dest = <any>{ ...opts1 };
    const source = <any>{ ...opts2 };

    // Ensure provider/providers are all expanded into the `ProviderResource[]` form.
    // This makes merging simple.
    expandProviders(dest);
    expandProviders(source);

    // iterate specifically over the supplied properties in [source].  Note: there may not be an
    // corresponding value in [dest].
    for (const key of Object.keys(source)) {
        const destVal = dest[key];
        const sourceVal = source[key];

        // For 'dependsOn' we might have singleton resources in both options bags. We
        // want to make sure we combine them into a collection.
        if (key === "dependsOn") {
            dest[key] = merge(destVal, sourceVal, /*alwaysCreateArray:*/ true);
            continue;
        }

        dest[key] = merge(destVal, sourceVal, /*alwaysCreateArray:*/ false);
    }

    // Now, if we are left with a .providers that is just a single key/value pair, then
    // collapse that down into .provider form.
    normalizeProviders(dest);

    return dest;
}

function isPromiseOrOutput(val: any): boolean {
    return val instanceof Promise || Output.isInstance(val);
}

/** @internal */
export function expandProviders(options: ComponentResourceOptions) {
    // Move 'provider' up to 'providers' if we have it.
    if (options.provider) {
        options.providers = [options.provider];
    }

    // Convert 'providers' map to array form.
    if (options.providers && !Array.isArray(options.providers)) {
        options.providers = utils.values(options.providers);
    }

    delete options.provider;
}

function normalizeProviders(opts: ComponentResourceOptions) {
    // If we have only 0-1 providers, then merge that back down to the .provider field.
    const providers = <ProviderResource[]>opts.providers;
    if (providers) {
        if (providers.length === 0) {
            delete opts.providers;
        }
        else if (providers.length === 1) {
            opts.provider = providers[0];
            delete opts.providers;
        }
        else {
            opts.providers = {};
            for (const res of providers) {
                opts.providers[res.getPackage()] = res;
            }
        }
    }
}

/** @internal for testing purposes. */
export function merge(dest: any, source: any, alwaysCreateArray: boolean): any {
    // unwind any top level promise/outputs.
    if (isPromiseOrOutput(dest)) {
        return output(dest).apply(d => merge(d, source, alwaysCreateArray));
    }

    if (isPromiseOrOutput(source)) {
        return output(source).apply(s => merge(dest, s, alwaysCreateArray));
    }

    // If either are an array, make a new array and merge the values into it.
    // Otherwise, just overwrite the destination with the source value.
    if (alwaysCreateArray || Array.isArray(dest) || Array.isArray(source)) {
        const result: any[] = [];
        addToArray(result, dest);
        addToArray(result, source);
        return result;
    }

    return source;
}

function addToArray(resultArray: any[], value: any) {
    if (Array.isArray(value)) {
        resultArray.push(...value);
    }
    else if (value !== undefined && value !== null) {
        resultArray.push(value);
    }
}

/**
 * A DependencyResource is a resource that is used to indicate that an Output has a dependency on a particular
 * resource. These resources are only created when dealing with remote component resources.
 */
export class DependencyResource extends CustomResource {
    constructor(urn: URN) {
        super("", "", {}, {}, true);

        (<any>this).urn = new Output(<any>this, Promise.resolve(urn), Promise.resolve(true), Promise.resolve(false),
            Promise.resolve([]));
    }
}

/**
 * A DependencyProviderResource is a resource that is used by the provider SDK as a stand-in for a provider that
 * is only used for its reference. Its only valid properties are its URN and ID.
 */
export class DependencyProviderResource extends ProviderResource {
    constructor(ref: string) {
        super("", "", {}, {}, true);

        // Parse the URN and ID out of the provider reference.
        const lastSep = ref.lastIndexOf("::");
        if (lastSep === -1) {
            throw new Error(`expected '::' in provider reference ${ref}`);
        }
        const urn = ref.slice(0, lastSep);
        const id = ref.slice(lastSep+2);

        (<any>this).urn = new Output(<any>this, Promise.resolve(urn), Promise.resolve(true), Promise.resolve(false), Promise.resolve([]));
        (<any>this).id = new Output(<any>this, Promise.resolve(id), Promise.resolve(true), Promise.resolve(false), Promise.resolve([]));
    }
}
