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

import * as url from "url";
import { ResourceError } from "./errors";
import * as log from "./log";
import { Input, Inputs, interpolate, Output, output } from "./output";
import {
    getResource,
    readResource,
    registerResource,
    registerResourceOutputs,
    SourcePosition,
} from "./runtime/resource";
import { unknownValue, SerializationOptions } from "./runtime/rpc";
import { getProject, getStack } from "./runtime/settings";
import { getStackResource } from "./runtime/state";
import * as utils from "./utils";

export type ID = string; // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * {@link createUrn} computes a URN from the combination of a resource name,
 * resource type, optional parent, optional project and optional stack.
 */
export function createUrn(
    name: Input<string>,
    type: Input<string>,
    parent?: Resource | Input<URN>,
    project?: string,
    stack?: string,
): Output<string> {
    let parentPrefix: Output<string>;
    if (parent) {
        let parentUrn: Output<string>;
        if (Resource.isInstance(parent)) {
            parentUrn = parent.urn;
        } else {
            parentUrn = output(parent);
        }
        parentPrefix = parentUrn.apply((parentUrnString) => {
            const prefix = parentUrnString.substring(0, parentUrnString.lastIndexOf("::")) + "$";
            if (prefix.endsWith("::pulumi:pulumi:Stack$")) {
                // Don't prefix the stack type as a parent type
                return `urn:pulumi:${stack || getStack()}::${project || getProject()}::`;
            }
            return prefix;
        });
    } else {
        parentPrefix = output(`urn:pulumi:${stack || getStack()}::${project || getProject()}::`);
    }
    return interpolate`${parentPrefix}${type}::${name}`;
}

/**
 * {@link inheritedChildAlias} computes the alias that should be applied to a
 * child based on an alias applied to its parent. This may involve changing the
 * name of the resource in cases where the resource has a named derived from the
 * name of the parent, and the parent name changed.
 */
function inheritedChildAlias(
    childName: string,
    parentName: string,
    parentAlias: Input<string>,
    childType: string,
): Output<string> {
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
        aliasName = output(parentAlias).apply((parentAliasUrn) => {
            const parentAliasName = parentAliasUrn.substring(parentAliasUrn.lastIndexOf("::") + 2);
            return parentAliasName + childName.substring(parentName.length);
        });
    }
    return createUrn(aliasName, childType, parentAlias);
}

/**
 * Extracts the type and name from a URN.
 */
function urnTypeAndName(urn: URN) {
    const parts = urn.split("::");
    const typeParts = parts[2].split("$");
    return {
        name: parts[3],
        type: typeParts[typeParts.length - 1],
    };
}

/**
 * {@link allAliases} computes the full set of aliases for a child resource
 * given a set of aliases applied to the child and parent resources. This
 * includes the child resource's own aliases, as well as aliases inherited from
 * the parent. If there are N child aliases, and M parent aliases, there will be
 * (M+1)*(N+1)-1 total aliases, or, as calculated in the logic below,
 * N+(M*(1+N)).
 */
export function allAliases(
    childAliases: Input<URN | Alias>[],
    childName: string,
    childType: string,
    parent: Resource,
    parentName: string,
): Output<URN>[] {
    const aliases: Output<URN>[] = [];
    for (const childAlias of childAliases) {
        aliases.push(collapseAliasToUrn(childAlias, childName, childType, parent));
    }
    for (const parentAlias of parent.__aliases || []) {
        // For each parent alias, add an alias that uses that base child name and the parent alias
        aliases.push(inheritedChildAlias(childName, parentName, parentAlias, childType));
        // Also add an alias for each child alias and the parent alias
        for (const childAlias of childAliases) {
            const inheritedAlias = collapseAliasToUrn(childAlias, childName, childType, parent).apply(
                (childAliasURN) => {
                    const { name: aliasedChildName, type: aliasedChildType } = urnTypeAndName(childAliasURN);
                    return inheritedChildAlias(aliasedChildName, parentName, parentAlias, aliasedChildType);
                },
            );
            aliases.push(inheritedAlias);
        }
    }
    return aliases;
}

/**
 * {@link Resource} represents a class whose CRUD operations are implemented by
 * a provider plugin.
 */
export abstract class Resource {
    /**
     * A regexp for use with {@link sourcePosition}.
     */
    private static sourcePositionRegExp =
        /Error:\s*\n\s*at new Resource \(.*\)\n\s*at new \S*Resource \(.*\)\n(\s*at new \S* \(.*\)\n)?[^(]*\((?<file>.*):(?<line>[0-9]+):(?<col>[0-9]+)\)\n/;

    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiResource: boolean = true;

    /**
     * The optional parent of this resource.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __parentResource: Resource | undefined;

    /**
     * The child resources of this resource. We use these (only from a
     * {@link ComponentResource}) to allow code to depend on a
     * {@link ComponentResource} and have that effectively mean that it is
     * depending on all the {@link CustomResource} children of that component.
     *
     * Important! We only walk through {@link ComponentResources}. They're the
     * only resources that serve as an aggregation of other primitive (i.e.
     * custom) resources. While a custom resource can be a parent of other
     * resources, we don't want to ever depend on those child resources. If we
     * do, it's simple to end up in a situation where we end up depending on a
     * child resource that has a data cycle dependency due to the data passed
     * into it.
     *
     * An example of how this would be bad is:
     *
     * ```ts
     *     var c1 = new CustomResource("c1");
     *     var c2 = new CustomResource("c2", { parentId: c1.id }, { parent: c1 });
     *     var c3 = new CustomResource("c3", { parentId: c1.id }, { parent: c1 });
     * ```
     *
     * The problem here is that 'c2' has a data dependency on 'c1'.  If it tries
     * to wait on `c1` it will walk to the children and wait on them. This will
     * mean it will wait on `c3`.  But `c3` will be waiting in the same manner
     * on `c2`, and a cycle forms.
     *
     * This normally does not happen with component resources as they do not
     * have any data flowing into them. The only way you would be able to have a
     * problem is if you had this sort of coding pattern:
     *
     * ```ts
     *     var c1 = new ComponentResource("c1");
     *     var c2 = new CustomResource("c2", { parentId: c1.urn }, { parent: c1 });
     *     var c3 = new CustomResource("c3", { parentId: c1.urn }, { parent: c1 });
     * ```
     *
     * However, this would be pretty nonsensical as there is zero need for a
     * custom resource to ever need to reference the URN of a component
     * resource. So it's acceptable if that sort of pattern fails in practice.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public __childResources: Set<Resource> | undefined;

    /**
     * The stable logical URN used to distinctly address a resource, both before
     * and after deployments.
     */
    public readonly urn!: Output<URN>;

    /**
     * When set to true, ensures that this resource cannot be deleted.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    private readonly __protect?: boolean;

    /**
     * A collection of transformations to apply as part of resource
     * registration.
     *
     * Note: This is marked optional only because older versions of this library
     * may not have had this property, and marking it as optional forces
     * consumers of the property to defensively handle cases where they are
     * passed "old" resources.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    __transformations?: ResourceTransformation[];

    /**
     * A list of aliases applied to this resource.
     *
     * Note: This is marked optional only because older versions of this library
     * may not have had this property, and marking it as optional forces
     * consumers of the property to defensively handle cases where they are
     * passed "old" resources.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    readonly __aliases?: Input<URN>[];

    /**
     * The name assigned to the resource at construction.
     *
     * Note: This is marked optional only because older versions of this library
     * may not have had this property, and marking it as optional forces
     * consumers of the property to defensively handle cases where they are
     * passed "old" resources.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    readonly __name?: string;

    /**
     * The set of providers to use for child resources. Keyed by package name
     * (e.g. "aws").
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    private readonly __providers: Record<string, ProviderResource>;

    /**
     * The specified provider or provider determined from the parent for custom
     * or remote resources. It is passed along in the `Call` gRPC request for
     * resource method calls (when set) so that the call goes to the same
     * provider as the resource.
     *
     * @internal
     */
    // Note: This is deliberately not named `__provider` as that conflicts with the property
    // used by the `dynamic.Resource` class.
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    readonly __prov?: ProviderResource;

    /**
     * The specified provider version.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    readonly __version?: string;

    /**
     * The specified provider download URL.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    readonly __pluginDownloadURL?: string;

    /**
     * A private field containing the type ID for this object. Useful for
     * implementing `isInstance` on classes that inherit from `Resource`.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiType: string;

    public static isInstance(obj: any): obj is Resource {
        return utils.isInstance<Resource>(obj, "__pulumiResource");
    }

    /**
     * Returns the source position of the user code that instantiated this
     * resource.
     *
     * This is somewhat brittle in that it expects a call stack of the form:
     *
     * - {@link Resource} class constructor
     * - abstract {@link Resource} subclass constructor
     * - concrete {@link Resource} subclass constructor
     * - user code
     *
     * This stack reflects the expected class hierarchy of:
     *
     * Resource > Custom/Component resource > Cloud/Component resource
     *
     * For example, consider the AWS S3 Bucket resource. When user code
     * instantiates a Bucket, the stack will look like this:
     *
     *     new Resource (/path/to/resource.ts:123:45)
     *     new CustomResource (/path/to/resource.ts:678:90)
     *     new Bucket (/path/to/bucket.ts:987:65)
     *     <user code> (/path/to/index.ts:4:3)
     *
     * Because Node can only give us the stack trace as text, we parse out the
     * source position using a regex that matches traces of this form (see
     * the {@link sourcePositionRegExp} above).
     */
    private static sourcePosition(): SourcePosition | undefined {
        const stackObj: any = {};
        Error.captureStackTrace(stackObj, Resource.sourcePosition);

        // Parse out the source position of the user code. If any part of the match is missing, return undefined.
        const { file, line, col } = Resource.sourcePositionRegExp.exec(stackObj.stack)?.groups || {};
        if (!file || !line || !col) {
            return undefined;
        }

        // Parse the line and column numbers. If either fails to parse, return undefined.
        //
        // Note: this really shouldn't happen given the regex; this is just a bit of defensive coding.
        const lineNum = parseInt(line, 10);
        const colNum = parseInt(col, 10);
        if (Number.isNaN(lineNum) || Number.isNaN(colNum)) {
            return undefined;
        }

        return {
            uri: url.pathToFileURL(file).toString(),
            line: lineNum,
            column: colNum,
        };
    }

    /**
     * Returns the provider for the given module member, if one exists.
     */
    public getProvider(moduleMember: string): ProviderResource | undefined {
        const pkg = pkgFromType(moduleMember);
        if (pkg === undefined) {
            return undefined;
        }

        return this.__providers[pkg];
    }

    /**
     * Creates and registers a new resource object. `t` is the fully qualified
     * type token and `name` is the "name" part to use in creating a stable and
     * globally unique URN for the object. `dependsOn` is an optional list of
     * other resources that this resource depends on, controlling the order in
     * which we perform resource operations.
     *
     * @param t
     *  The type of the resource.
     * @param name
     *  The _unique_ name of the resource.
     * @param custom
     *  True to indicate that this is a custom resource, managed by a plugin.
     * @param props
     *  The arguments to use to populate the new resource.
     * @param opts
     *  A bag of options that control this resource's behavior.
     * @param remote
     *  True if this is a remote component resource.
     * @param dependency
     *  True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(
        t: string,
        name: string,
        custom: boolean,
        props: Inputs = {},
        opts: ResourceOptions = {},
        remote: boolean = false,
        dependency: boolean = false,
        packageRef?: Promise<string | undefined>,
    ) {
        this.__pulumiType = t;

        if (dependency) {
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
        const parent = opts.parent || getStackResource();
        this.__transformations = [...(opts.transformations || []), ...(parent?.__transformations || [])];
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

        // Check the parent type if one exists and fill in any default options.
        this.__providers = {};
        if (parent) {
            this.__parentResource = parent;
            this.__parentResource.__childResources = this.__parentResource.__childResources || new Set();
            this.__parentResource.__childResources.add(this);

            if (opts.protect === undefined) {
                opts.protect = parent.__protect;
            }

            this.__providers = parent.__providers;
        }

        // providers is found by combining (in ascending order of priority)
        //      1. provider
        //      2. self_providers
        //      3. opts.providers
        this.__providers = {
            ...this.__providers,
            ...convertToProvidersMap((<ComponentResourceOptions>opts).providers),
            ...convertToProvidersMap(opts.provider ? [opts.provider] : {}),
        };

        const pkg = pkgFromType(t);

        // provider is the first option that does not return none
        // 1. opts.provider
        // 2. a matching provider in opts.providers
        // 3. a matching provider inherited from opts.parent
        if ((custom || remote) && opts.provider === undefined) {
            const parentProvider = parent?.getProvider(t);

            if (pkg && pkg in this.__providers) {
                opts.provider = this.__providers[pkg];
            } else if (parentProvider) {
                opts.provider = parentProvider;
            }
        }

        // Custom and remote resources have a backing provider. If this is a custom or
        // remote resource and a provider has been specified that has the same package
        // as the resource's package, save it in `__prov`.
        // If the provider's package isn't the same as the resource's package, don't
        // save it in `__prov` because the user specified `provider: someProvider` as
        // shorthand for `providers: [someProvider]`, which is a provider intended for
        // the component's children and not for this resource itself.
        // `__prov` is passed along in `Call` gRPC requests for resource method calls
        // (when set) so that the call goes to the same provider as the resource.
        if ((custom || remote) && opts.provider) {
            if (pkg && pkg === opts.provider.getPackage()) {
                this.__prov = opts.provider;
            }
        }

        this.__protect = opts.protect;
        this.__version = opts.version;
        this.__pluginDownloadURL = opts.pluginDownloadURL;

        // Collapse any `Alias`es down to URNs. We have to wait until this point to do so because we do not know the
        // default `name` and `type` to apply until we are inside the resource constructor.
        this.__aliases = [];
        if (opts.aliases) {
            for (const alias of opts.aliases) {
                this.__aliases.push(collapseAliasToUrn(alias, name, t, parent));
            }
        }

        const sourcePosition = Resource.sourcePosition();

        if (opts.urn) {
            // This is a resource that already exists. Read its state from the engine.
            getResource(this, parent, props, custom, opts.urn);
        } else if (opts.id) {
            // If this is a custom resource that already exists, read its state from the provider.
            if (!custom) {
                throw new ResourceError(
                    "Cannot read an existing resource unless it has a custom provider",
                    opts.parent,
                );
            }
            readResource(this, parent, t, name, props, opts, sourcePosition, packageRef);
        } else {
            // Kick off the resource registration.  If we are actually performing a deployment, this
            // resource's properties will be resolved asynchronously after the operation completes, so
            // that dependent computations resolve normally.  If we are just planning, on the other
            // hand, values will never resolve.
            registerResource(
                this,
                parent,
                t,
                name,
                custom,
                remote,
                (urn) => new DependencyResource(urn),
                props,
                opts,
                sourcePosition,
                packageRef,
            );
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
 * A constant to represent the "root stack" resource for a Pulumi application.
 * The purpose of this is solely to make it easy to write an {@link Alias} like
 * so:
 *
 * `aliases: [{ parent: rootStackResource }]`.
 *
 * This indicates that the prior name for a resource was created based on it
 * being parented directly by the stack itself and no other resources. Note:
 * this is equivalent to:
 *
 * `aliases: [{ parent: undefined }]`
 *
 * However, the former form is preferable as it is more self-descriptive, while
 * the latter may look a bit confusing and may incorrectly look like something
 * that could be removed without changing semantics.
 */
export const rootStackResource: Resource = undefined!;

/**
 * {@link Alias} is a partial description of prior names used for a resource. It
 * can be processed in the context of a resource creation to determine what the
 * full aliased URN would be.
 *
 * Note there is a semantic difference between properties being absent from this
 * type and properties having the `undefined` value. Specifically, there is a
 * difference between:
 *
 * ```ts
 * { name: "foo", parent: undefined } // and
 * { name: "foo" }
 * ```
 *
 * The presence of a property indicates if its value should be used.  If absent,
 * then the value is not used. So, in the above while `alias.parent` is
 * `undefined` for both, the first alias means "the original URN had no parent"
 * while the second alias means "use the current parent".
 *
 * Note: to indicate that a resource was previously parented by the root stack,
 * it is recommended that you use:
 *
 * `aliases: [{ parent: pulumi.rootStackResource }]`
 *
 * This form is self-descriptive and makes the intent clearer than using:
 *
 * `aliases: [{ parent: undefined }]`
 */
export interface Alias {
    /**
     * The previous name of the resource. If not provided, the current name of
     * the resource is used.
     */
    name?: Input<string>;

    /**
     * The previous type of the resource. If not provided, the current type of
     * the resource is used.
     */
    type?: Input<string>;

    /**
     * The previous parent of the resource. If not provided (i.e. `{ name: "foo"
     * }`), the current parent of the resource is used (`opts.parent` if
     * provided, else the implicit stack resource parent).
     *
     * To specify no original parent, use `{ parent: pulumi.rootStackResource }`.
     */
    parent?: Resource | Input<URN>;

    /**
     * The previous stack of the resource. If not provided, defaults to
     * `pulumi.getStack()`.
     */
    stack?: Input<string>;

    /**
     * The previous project of the resource. If not provided, defaults to
     * `pulumi.getProject()`.
     */
    project?: Input<string>;
}

/**
 * Converts an alias into a URN given a set of default data for the missing
 * values.
 */
function collapseAliasToUrn(
    alias: Input<Alias | string>,
    defaultName: string,
    defaultType: string,
    defaultParent: Resource | undefined,
): Output<URN> {
    return output(alias).apply((a) => {
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
 * {@link ResourceOptions} is a bag of optional settings that control a
 * resource's behavior.
 */
export interface ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies that
    // mergeOptions works properly for it. Also be sure to update the logic in callbacks.ts that marshals to
    // and from this type to the wire protocol.

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
     * Optional list of transforms to apply to this resource during construction. The
     * transforms are applied in order, and are applied prior to transforms applied to
     * parents walking from the resource up to the stack.
     *
     * This property is experimental.
     */
    transforms?: ResourceTransform[];

    /**
     * The URN of a previously-registered resource of this type to read from the engine.
     */
    urn?: URN;

    /**
     * An option to specify the URL from which to download this resources
     * associated plugin. This version overrides the URL information inferred
     * from the current package and should rarely be used.
     */
    pluginDownloadURL?: string;

    /**
     * If set to True, the providers Delete method will not be called for this resource.
     */
    retainOnDelete?: boolean;

    /**
     * If set, the providers Delete method will not be called for this resource
     * if specified is being deleted as well.
     */
    deletedWith?: Resource;

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
 * {@link ResourceTransformation} is the callback signature for the
 * `transformations` resource option.  A transformation is passed the same set
 * of inputs provided to the {@link Resource} constructor, and can optionally
 * return back alternate values for the `props` and/or `opts` prior to the
 * resource actually being created.  The effect will be as though those props
 * and opts were passed in place of the original call to the {@link Resource}
 * constructor. If the transformation returns `undefined`, this indicates that
 * the resource will not be transformed.
 */
export type ResourceTransformation = (args: ResourceTransformationArgs) => ResourceTransformationResult | undefined;

/**
 * {@link ResourceTransform} is the callback signature for the `transforms`
 * resource option.  A transform is passed the same set of inputs provided to
 * the {@link Resource} constructor, and can optionally return back alternate
 * values for the `props` and/or `opts` prior to the resource actually being
 * created.  The effect will be as though those props and opts were passed in
 * place of the original call to the {@link Resource} constructor.  If the
 * transform returns `undefined`, this indicates that the resource will not be
 * transformed.
 */
export type ResourceTransform = (
    args: ResourceTransformArgs,
) => Promise<ResourceTransformResult | undefined> | ResourceTransformResult | undefined;

/**
 * {@link ResourceTransformArgs} is the argument bag passed to a resource transform.
 */
export interface ResourceTransformArgs {
    /**
     * True if the resource is a custom resource, false if it is a component resource.
     */
    custom: boolean;

    /**
     * The type of the resource.
     */
    type: string;

    /**
     * The name of the resource.
     */
    name: string;

    /**
     * The original properties passed to the resource constructor.
     */
    props: Inputs;

    /**
     * The original resource options passed to the resource constructor.
     */
    opts: ResourceOptions;
}
/**
 * {@link ResourceTransformResult} is the result that must be returned by a
 * resource transform callback.  It includes new values to use for the
 * `props` and `opts` of the {@link Resource} in place of the originally
 * provided values.
 */
export interface ResourceTransformResult {
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
 * {@link ResourceTransformationArgs} is the argument bag passed to a resource
 * transformation.
 */
export interface ResourceTransformationArgs {
    /**
     * The {@link Resource} instance that is being transformed.
     */
    resource: Resource;

    /**
     * The type of the resource.
     */
    type: string;

    /**
     * The name of the resource.
     */
    name: string;

    /**
     * The original properties passed to the resource constructor.
     */
    props: Inputs;

    /**
     * The original resource options passed to the resource constructor.
     */
    opts: ResourceOptions;
}

/**
 * {@link ResourceTransformationResult} is the result that must be returned by a
 * resource transformation callback.  It includes new values to use for the
 * `props` and `opts` of the {@link Resource} in place of the originally
 * provided values.
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
 * {@link CustomResourceOptions} is a bag of optional settings that control a
 * custom resource's behavior.
 */
export interface CustomResourceOptions extends ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.

    /**
     * When set to true, indicates that this resource should be deleted before
     * its replacement is created when replacement is necessary.
     */
    deleteBeforeReplace?: boolean;

    /**
     * The names of outputs for this resource that should be treated as secrets.
     * This augments the list that the resource provider and Pulumi engine
     * already determine based on inputs to your resource. It can be used to
     * mark certain ouputs as a secrets on a per resource basis.
     */
    additionalSecretOutputs?: string[];

    /**
     * When provided with a resource ID, indicates that this resource's provider
     * should import its state from the cloud resource with the given ID. The
     * inputs to the resource's constructor must align with the resource's
     * current state. Once a resource has been imported, the import property
     * must be removed from the resource's options.
     */
    import?: ID;

    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.
}

/**
 * {@link ComponentResourceOptions} is a bag of optional settings that control a
 * component resource's behavior.
 */
export interface ComponentResourceOptions extends ResourceOptions {
    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.

    /**
     * An optional set of providers to use for child resources. Either keyed by
     * package name (e.g. "aws"), or just provided as an array.  In the latter
     * case, the package name will be retrieved from the provider itself.
     *
     * Note: only a list should be used. Mapping keys are not respected.
     */
    providers?: Record<string, ProviderResource> | ProviderResource[];

    // !!! IMPORTANT !!! If you add a new field to this type, make sure to add test that verifies
    // that mergeOptions works properly for it.
}

/**
 * {@link CustomResource} is a resource whose create, read, update, and delete
 * (CRUD) operations are managed by performing external operations on some
 * physical entity.  The engine understands how to diff and perform partial
 * updates of them, and these CRUD operations are implemented in a dynamically
 * loaded plugin for the defining package.
 */
export abstract class CustomResource extends Resource {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     * @internal
     *
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiCustomResource: boolean;

    /**
     * The provider-assigned unique ID for this managed resource. It is set
     * during deployments and may be missing (undefined) during planning phases.
     */
    public readonly id!: Output<ID>;

    /**
     * Returns true if the given object is a {@link CustomResource}. This is
     * designed to work even when multiple copies of the Pulumi SDK have been
     * loaded into the same process.
     */
    public static isInstance(obj: any): obj is CustomResource {
        return utils.isInstance<CustomResource>(obj, "__pulumiCustomResource");
    }

    /**
     * Creates and registers a new managed resource. `t` is the fully qualified
     * type token and `name` is the "name" part to use in creating a stable and
     * globally unique URN for the object. `dependsOn` is an optional list of
     * other resources that this resource depends on, controlling the order in
     * which we perform resource operations. Creating an instance does not
     * necessarily perform a create on the physical entity which it represents.
     * Instead, this is dependent upon the diffing of the new goal state
     * compared to the current known resource state.
     *
     * @param t
     *  The type of the resource.
     * @param name
     *  The _unique_ name of the resource.
     * @param props
     *  The arguments to use to populate the new resource.
     * @param opts
     *  A bag of options that control this resource's behavior.
     * @param dependency
     *  True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(
        t: string,
        name: string,
        props?: Inputs,
        opts: CustomResourceOptions = {},
        dependency = false,
        packageRef?: Promise<string | undefined>,
    ) {
        if ((<ComponentResourceOptions>opts).providers) {
            throw new ResourceError(
                "Do not supply 'providers' option to a CustomResource. Did you mean 'provider' instead?",
                opts.parent,
            );
        }

        super(t, name, true, props, opts, false, dependency, packageRef);
        this.__pulumiCustomResource = true;
    }
}

(<any>CustomResource).doNotCapture = true;

/**
 * {@link ProviderResource} is a resource that implements CRUD operations for
 * other custom resources. These resources are managed similarly to other
 * resources, including the usual diffing and update semantics.
 */
export abstract class ProviderResource extends CustomResource {
    /**
     * @internal
     */
    private readonly pkg: string;

    /**
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public __registrationId?: string;

    public static async register(provider: ProviderResource | undefined): Promise<string | undefined> {
        if (provider === undefined) {
            return undefined;
        }

        if (!provider.__registrationId) {
            const providerURN = await provider.urn.promise();
            const providerID = (await provider.id.promise()) || unknownValue;
            provider.__registrationId = `${providerURN}::${providerID}`;
        }

        return provider.__registrationId;
    }

    /**
     * Creates and registers a new provider resource for a particular package.
     *
     * @param pkg
     *  The package associated with this provider.
     * @param name
     *  The _unique_ name of the provider.
     * @param props
     *  The configuration to use for this provider.
     * @param opts
     *  A bag of options that control this provider's behavior.
     * @param dependency
     *  True if this is a synthetic resource used internally for dependency tracking.
     */
    constructor(
        pkg: string,
        name: string,
        props?: Inputs,
        opts: ResourceOptions = {},
        dependency: boolean = false,
        packageRef?: Promise<string | undefined>,
    ) {
        super(`pulumi:providers:${pkg}`, name, props, opts, dependency, packageRef);
        this.pkg = pkg;
    }

    /**
     * @internal
     */
    public getPackage() {
        return this.pkg;
    }
}

/**
 * {@link ComponentResource} is a resource that aggregates one or more other
 * child resources into a higher level abstraction. The component resource
 * itself is a resource, but does not require custom CRUD operations for
 * provisioning.
 */
export class ComponentResource<TData = any> extends Resource {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiComponentResource = true;

    /**
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __data: Promise<TData>;

    /**
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    private __registered = false;

    /**
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __remote: boolean;

    /**
     * Returns true if the given object is a {@link CustomResource}. This is
     * designed to work even when multiple copies of the Pulumi SDK have been
     * loaded into the same process.
     */
    public static isInstance(obj: any): obj is ComponentResource {
        return utils.isInstance<ComponentResource>(obj, "__pulumiComponentResource");
    }

    /**
     * Creates and registers a new component resource. `type` is the fully
     * qualified type token and `name` is the "name" part to use in creating a
     * stable and globally unique URN for the object. `opts.parent` is the
     * optional parent for this component, and `opts.dependsOn` is an optional
     * list of other resources that this resource depends on, controlling the
     * order in which we perform resource operations.
     *
     * @param type
     *  The type of the resource.
     * @param name
     *  The _unique_ name of the resource.
     * @param args
     *  Information passed to [initialize] method.
     * @param opts
     *  A bag of options that control this resource's behavior.
     * @param remote
     *  True if this is a remote component resource.
     */
    constructor(
        type: string,
        name: string,
        args: Inputs = {},
        opts: ComponentResourceOptions = {},
        remote: boolean = false,
        packageRef?: Promise<string | undefined>,
    ) {
        // Explicitly ignore the props passed in.  We allow them for back compat reasons.  However,
        // we explicitly do not want to pass them along to the engine.  The ComponentResource acts
        // only as a container for other resources.  Another way to think about this is that a normal
        // 'custom resource' corresponds to real piece of cloud infrastructure.  So, when it changes
        // in some way, the cloud resource needs to be updated (and vice versa).  That is not true
        // for a component resource.  The component is just used for organizational purposes and does
        // not correspond to a real piece of cloud infrastructure.  As such, changes to it *itself*
        // do not have any effect on the cloud side of things at all.
        super(
            type,
            name,
            /*custom:*/ false,
            /*props:*/ remote || opts?.urn ? args : {},
            opts,
            remote,
            false,
            packageRef,
        );
        this.__remote = remote;
        this.__registered = remote || !!opts?.urn;
        this.__data = remote || opts?.urn ? Promise.resolve(<TData>{}) : this.initializeAndRegisterOutputs(args);
    }

    /**
     * @internal
     */
    private async initializeAndRegisterOutputs(args: Inputs) {
        const data = await this.initialize(args);
        this.registerOutputs();
        return data;
    }

    /**
     * Can be overridden by a subclass to asynchronously initialize data for this component
     * automatically when constructed. The data will be available immediately for subclass
     * constructors to use. To access the data use {@link getData}.
     */
    protected async initialize(args: Inputs): Promise<TData> {
        return <TData>undefined!;
    }

    /**
     * Retrieves the data produces by {@link initialize}. The data is
     * immediately available in a derived class's constructor after the
     * `super(...)` call to `ComponentResource`.
     */
    protected getData(): Promise<TData> {
        return this.__data;
    }

    /**
     * Registers synthetic outputs that a component has initialized, usually by
     * allocating other child sub-resources and propagating their resulting
     * property values.
     *
     * Component resources can call this at the end of their constructor to
     * indicate that they are done creating child resources.  This is not
     * strictly necessary as this will automatically be called after the {@link
     * initialize} method completes.
     */
    protected registerOutputs(outputs?: Inputs | Promise<Inputs> | Output<Inputs>, opts?: SerializationOptions): void {
        if (this.__registered) {
            return;
        }

        this.__registered = true;
        registerResourceOutputs(this, outputs || {}, opts);
    }
}

(<any>ComponentResource).doNotCapture = true;
(<any>ComponentResource.prototype).registerOutputs.doNotCapture = true;
(<any>ComponentResource.prototype).initialize.doNotCapture = true;
(<any>ComponentResource.prototype).initializeAndRegisterOutputs.doNotCapture = true;

/**
 * @internal
 */
export const testingOptions = {
    isDryRun: false,
};

/**
 * {@link mergeOptions} takes two {@link ResourceOptions} values and produces a new
 * {@link ResourceOptions} with the respective properties of `opts2` merged over the
 * same properties in `opts1`. The original options objects will be unchanged.
 *
 * Conceptually property merging follows these basic rules:
 *
 *  1. if the property is a collection, the final value will be a collection
 *     containing the values from each options object.
 *
 *  2. Simple scaler values from `opts2` (i.e. strings, numbers, bools) will
 *     replace the values of `opts1`.
 *
 *  3. `opts2` can have properties explicitly provided with `null` or
 *     `undefined` as the value. If explicitly provided, then that will be the
 *     final value in the result.
 *
 *  4. For the purposes of merging `dependsOn`, `provider` and `providers` are
 *     always treated as collections, even if only a single value was provided.
 */
export function mergeOptions(
    opts1: CustomResourceOptions | undefined,
    opts2: CustomResourceOptions | undefined,
): CustomResourceOptions;
export function mergeOptions(
    opts1: ComponentResourceOptions | undefined,
    opts2: ComponentResourceOptions | undefined,
): ComponentResourceOptions;
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

/**
 * @internal
 */
export function expandProviders(options: ComponentResourceOptions) {
    // Convert 'providers' map to array form.
    if (options.providers && !Array.isArray(options.providers)) {
        for (const k in options.providers) {
            if (Object.prototype.hasOwnProperty.call(options.providers, k)) {
                const v = options.providers[k];
                if (k !== v.getPackage()) {
                    const message = `provider resource map where key ${k} doesn't match provider ${v.getPackage()}`;
                    log.warn(message);
                }
            }
        }
        options.providers = utils.values(options.providers);
    }
}

function normalizeProviders(opts: ComponentResourceOptions) {
    // If we have 0 providers, delete providers. Otherwise, convert providers into a map.
    const providers = <ProviderResource[]>opts.providers;
    if (providers) {
        if (providers.length === 0) {
            opts.providers = undefined;
        } else {
            opts.providers = {};
            for (const res of providers) {
                opts.providers[res.getPackage()] = res;
            }
        }
    }
}

/**
 * @internal
 *  This is exported for testing purposes.
 */
export function merge(dest: any, source: any, alwaysCreateArray: boolean): any {
    // unwind any top level promise/outputs.
    if (isPromiseOrOutput(dest)) {
        return output(dest).apply((d) => merge(d, source, alwaysCreateArray));
    }

    if (isPromiseOrOutput(source)) {
        return output(source).apply((s) => merge(dest, s, alwaysCreateArray));
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
    } else if (value !== undefined && value !== null) {
        resultArray.push(value);
    }
}

/**
 * A {@link DependencyResource} is a resource that is used to indicate that an
 * {@link Output} has a dependency on a particular resource. These resources are
 * only created when dealing with remote component resources.
 */
export class DependencyResource extends CustomResource {
    constructor(urn: URN) {
        super("", "", {}, {}, true);

        (<any>this).urn = new Output(
            <any>this,
            Promise.resolve(urn),
            Promise.resolve(true),
            Promise.resolve(false),
            Promise.resolve([]),
        );
    }
}

/**
 * A {@link DependencyProviderResource} is a resource that is used by the
 * provider SDK as a stand-in for a provider that is only used for its
 * reference. Its only valid properties are its URN and ID.
 */
export class DependencyProviderResource extends ProviderResource {
    constructor(ref: string) {
        const [urn, id] = parseResourceReference(ref);
        const urnParts = urn.split("::");
        const qualifiedType = urnParts[2];
        const type = qualifiedType.split("$").pop()!;
        // type will be "pulumi:providers:<package>" and we want the last part.
        const typeParts = type.split(":");
        const pkg = typeParts.length > 2 ? typeParts[2] : "";

        super(pkg, "", {}, {}, true);

        (<any>this).urn = new Output(
            <any>this,
            Promise.resolve(urn),
            Promise.resolve(true),
            Promise.resolve(false),
            Promise.resolve([]),
        );
        (<any>this).id = new Output(
            <any>this,
            Promise.resolve(id),
            Promise.resolve(true),
            Promise.resolve(false),
            Promise.resolve([]),
        );
    }
}

/**
 * Parses the URN and ID out of the provider reference.
 *
 * @internal
 */
export function parseResourceReference(ref: string): [string, string] {
    const lastSep = ref.lastIndexOf("::");
    if (lastSep === -1) {
        throw new Error(`expected '::' in provider reference ${ref}`);
    }
    const urn = ref.slice(0, lastSep);
    const id = ref.slice(lastSep + 2);
    return [urn, id];
}

/**
 * Extracts the package from the type token of the form "pkg:module:member".
 *
 * @internal
 */
export function pkgFromType(type: string): string | undefined {
    const parts = type.split(":");
    if (parts.length === 3) {
        return parts[0];
    }
    return undefined;
}

/**
 * The Pulumi type assigned to the resource at construction, of the form `package:module:name`.
 */
export function resourceType(res: Resource): string {
    return res.__pulumiType;
}

/**
 * The Pulumi name assigned to the resource at construction, i.e. the "name" in its constructor call.
 */
export function resourceName(res: Resource): string {
    if (res.__name === undefined) {
        throw new ResourceError(
            "Resource name is not available, this resource instance must have been constructed by an old SDK",
            res,
        );
    }
    return res.__name;
}
