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

import * as grpc from "grpc";
import * as log from "../log";
import { Input, Inputs, Output } from "../output";
import {
    ComponentResource,
    CustomResource,
    CustomResourceOptions,
    ID,
    Resource,
    ResourceOptions,
    URN,
} from "../resource";
import { debuggablePromise } from "./debuggable";

import {
    deserializeProperties,
    deserializeProperty,
    OutputResolvers,
    resolveProperties,
    serializeProperties,
    serializeProperty,
    serializeResourceProperties,
    transferProperties,
    unknownValue,
} from "./rpc";
import {
    excessiveDebugOutput,
    getMonitor,
    getProject,
    getRootResource,
    getStack,
    rpcKeepAlive,
    serialize,
} from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const resproto = require("../proto/resource_pb.js");

interface ResourceResolverOperation {
    // A resolver for a resource's URN.
    resolveURN: (urn: URN) => void;
    // A resolver for a resource's ID (for custom resources only).
    resolveID: ((v: ID, performApply: boolean) => void) | undefined;
    // A collection of resolvers for a resource's properties.
    resolvers: OutputResolvers;
    // A parent URN, fully resolved, if any.
    parentURN: URN | undefined;
    // A provider reference, fully resolved, if any.
    providerRef: string | undefined;
    // All serialized properties, fully awaited, serialized, and ready to go.
    serializedProps: Record<string, any>;
    // A set of URNs that this resource is directly dependent upon.  These will all be URNs of
    // custom resources, not component resources.
    allDirectDependencyURNs: Set<URN>;
    // Set of URNs that this resource is directly dependent upon, keyed by the property that causes
    // the dependency.  All urns in this map must exist in [allDirectDependencyURNs].  These will
    // all be URNs of custom resources, not component resources.
    propertyToDirectDependencyURNs: Map<string, Set<URN>>;
}

/**
 * Creates a test URN in the case where the engine isn't available to give us one.
 */
function createTestUrn(t: string, name: string): string {
    return `urn:pulumi:${getStack()}::${getProject()}::${t}::${name}`;
}

/**
 * Reads an existing custom resource's state from the resource monitor.  Note that resources read in this way
 * will not be part of the resulting stack's state, as they are presumed to belong to another.
 */
export function readResource(res: Resource, t: string, name: string, props: Inputs, opts: ResourceOptions): void {
    const id: Input<ID> | undefined = opts.id;
    if (!id) {
        throw new Error("Cannot read resource whose options are lacking an ID value");
    }

    const label = `resource:${name}[${t}]#...`;
    log.debug(`Reading resource: id=${Output.isInstance(id) ? "Output<T>" : id}, t=${t}, name=${name}`);

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, true, props, opts);

    const preallocError = new Error();
    debuggablePromise(resopAsync.then(async (resop) => {
        const resolvedID = await serializeProperty(label, id, new Set());
        log.debug(`ReadResource RPC prepared: id=${resolvedID}, t=${t}, name=${name}` +
            (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``));

        // Create a resource request and do the RPC.
        const req = new resproto.ReadResourceRequest();
        req.setType(t);
        req.setName(name);
        req.setId(resolvedID);
        req.setParent(resop.parentURN);
        req.setProvider(resop.providerRef);
        req.setProperties(gstruct.Struct.fromJavaScript(resop.serializedProps));
        req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
        req.setVersion(opts.version || "");
        req.setAcceptsecrets(true);

        // Now run the operation, serializing the invocation if necessary.
        const opLabel = `monitor.readResource(${label})`;
        runAsyncResourceOp(opLabel, async () => {
            let resp: any;
            if (monitor) {
                // If we're attached to the engine, make an RPC call and wait for it to resolve.
                resp = await debuggablePromise(new Promise((resolve, reject) =>
                    (monitor as any).readResource(req, (err: Error, innerResponse: any) => {
                        log.debug(`ReadResource RPC finished: ${label}; err: ${err}, resp: ${innerResponse}`);
                        if (err) {
                            preallocError.message =
                                `failed to read resource #${resolvedID} '${name}' [${t}]: ${err.message}`;
                            reject(preallocError);
                        }
                        else {
                            resolve(innerResponse);
                        }
                    })), opLabel);
            } else {
                // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
                resp = {
                    getUrn: () => createTestUrn(t, name),
                    getProperties: () => req.getProperties(),
                };
            }

            // Now resolve everything: the URN, the ID (supplied as input), and the output properties.
            resop.resolveURN(resp.getUrn());
            resop.resolveID!(resolvedID, resolvedID !== undefined);
            await resolveOutputs(res, t, name, props, resp.getProperties(), resop.resolvers);
        });
    }), label);
}

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
 * URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, custom: boolean,
                                 props: Inputs, opts: ResourceOptions): void {
    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}`);

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, custom, props, opts);

    // In order to present a useful stack trace if an error does occur, we preallocate potential
    // errors here. V8 captures a stack trace at the moment an Error is created and this stack
    // trace will lead directly to user code. Throwing in `runAsyncResourceOp` results in an Error
    // with a non-useful stack trace.
    const preallocError = new Error();
    debuggablePromise(resopAsync.then(async (resop) => {
        log.debug(`RegisterResource RPC prepared: t=${t}, name=${name}` +
            (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``));

        const req = new resproto.RegisterResourceRequest();
        req.setType(t);
        req.setName(name);
        req.setParent(resop.parentURN);
        req.setCustom(custom);
        req.setObject(gstruct.Struct.fromJavaScript(resop.serializedProps));
        req.setProtect(opts.protect);
        req.setProvider(resop.providerRef);
        req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
        req.setDeletebeforereplace((<any>opts).deleteBeforeReplace || false);
        req.setIgnorechangesList(opts.ignoreChanges || []);
        req.setVersion(opts.version || "");
        req.setAcceptsecrets(true);
        req.setAdditionalsecretoutputsList((<any>opts).additionalSecretOutputs || []);

        const propertyDependencies = req.getPropertydependenciesMap();
        for (const [key, resourceURNs] of resop.propertyToDirectDependencyURNs) {
            const deps = new resproto.RegisterResourceRequest.PropertyDependencies();
            deps.setUrnsList(Array.from(resourceURNs));
            propertyDependencies.set(key, deps);
        }

        // Now run the operation, serializing the invocation if necessary.
        const opLabel = `monitor.registerResource(${label})`;
        runAsyncResourceOp(opLabel, async () => {
            let resp: any;
            if (monitor) {
                // If we're running with an attachment to the engine, perform the operation.
                resp = await debuggablePromise(new Promise((resolve, reject) =>
                    (monitor as any).registerResource(req, (err: grpc.ServiceError, innerResponse: any) => {
                        log.debug(`RegisterResource RPC finished: ${label}; err: ${err}, resp: ${innerResponse}`);
                        if (err) {
                            // If the monitor is unavailable, it is in the process of shutting down or has already
                            // shut down. Don't emit an error and don't do any more RPCs, just exit.
                            if (err.code === grpc.status.UNAVAILABLE) {
                                log.debug("Resource monitor is terminating");
                                process.exit(0);
                            }

                            // Node lets us hack the message as long as we do it before accessing the `stack` property.
                            preallocError.message = `failed to register new resource ${name} [${t}]: ${err.message}`;
                            reject(preallocError);
                        }
                        else {
                            resolve(innerResponse);
                        }
                    })), opLabel);
            } else {
                // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
                resp = {
                    getUrn: () => createTestUrn(t, name),
                    getId: () => undefined,
                    getObject: () => req.getObject(),
                };
            }

            resop.resolveURN(resp.getUrn());

            // Note: 'id || undefined' is intentional.  We intentionally collapse falsy values to
            // undefined so that later parts of our system don't have to deal with values like 'null'.
            if (resop.resolveID) {
                const id = resp.getId() || undefined;
                resop.resolveID(id, id !== undefined);
            }

            // Now resolve the output properties.
            await resolveOutputs(res, t, name, props, resp.getObject(), resop.resolvers);
        });
    }), label);
}

/**
 * Prepares for an RPC that will manufacture a resource, and hence deals with input and output
 * properties.
 */
async function prepareResource(label: string, res: Resource, custom: boolean,
                               props: Inputs, opts: ResourceOptions): Promise<ResourceResolverOperation> {

    // Simply initialize the URN property and get prepared to resolve it later on.
    // Note: a resource urn will always get a value, and thus the output property
    // for it can always run .apply calls.
    let resolveURN: (urn: URN) => void;
    (res as any).urn = new Output(
        res,
        debuggablePromise(
            new Promise<URN>(resolve => resolveURN = resolve),
            `resolveURN(${label})`),
        /*isKnown:*/ Promise.resolve(true),
        /*isSecret:*/ Promise.resolve(false));

    // If a custom resource, make room for the ID property.
    let resolveID: ((v: any, performApply: boolean) => void) | undefined;
    if (custom) {
        let resolveValue: (v: ID) => void;
        let resolveIsKnown: (v: boolean) => void;
        (res as any).id = new Output(
            res,
            debuggablePromise(new Promise<ID>(resolve => resolveValue = resolve), `resolveID(${label})`),
            debuggablePromise(new Promise<boolean>(
                resolve => resolveIsKnown = resolve), `resolveIDIsKnown(${label})`),
            Promise.resolve(false));

        resolveID = (v, isKnown) => {
            resolveValue(v);
            resolveIsKnown(isKnown);
        };
    }

    // Now "transfer" all input properties into unresolved Promises on res.  This way,
    // this resource will look like it has all its output properties to anyone it is
    // passed to.  However, those promises won't actually resolve until the registerResource
    // RPC returns
    const resolvers = transferProperties(res, label, props);

    /** IMPORTANT!  We should never await prior to this line, otherwise the Resource will be partly uninitialized. */

    // Before we can proceed, all our dependencies must be finished.
    const explicitDirectDependencies = new Set(await gatherExplicitDependencies(opts.dependsOn));

    // Serialize out all our props to their final values.  In doing so, we'll also collect all
    // the Resources pointed to by any Dependency objects we encounter, adding them to 'propertyDependencies'.
    const [serializedProps, propertyToDirectDependencies] = await serializeResourceProperties(label, props);

    // Wait for the parent to complete.
    // If no parent was provided, parent to the root resource.
    const parentURN = opts.parent
        ? await opts.parent.urn.promise()
        : await getRootResource();

    let providerRef: string | undefined;
    if (custom && (<CustomResourceOptions>opts).provider) {
        const provider = (<CustomResourceOptions>opts).provider!;
        const providerURN = await provider.urn.promise();
        const providerID = await provider.id.promise() || unknownValue;
        providerRef = `${providerURN}::${providerID}`;
    }

    // Collect the URNs for explicit/implicit dependencies for the engine so that it can understand
    // the dependency graph and optimize operations accordingly.

    // The list of all dependencies (implicit or explicit).
    const allDirectDependencies = new Set<Resource>(explicitDirectDependencies);

    const allDirectDependencyURNs = await getAllTransitivelyReferencedCustomResourceURNs(explicitDirectDependencies);
    const propertyToDirectDependencyURNs = new Map<string, Set<URN>>();

    for (const [propertyName, directDependencies] of propertyToDirectDependencies) {
        addAll(allDirectDependencies, directDependencies);

        const urns = await getAllTransitivelyReferencedCustomResourceURNs(directDependencies);
        addAll(allDirectDependencyURNs, urns);
        propertyToDirectDependencyURNs.set(propertyName, urns);
    }

    return {
        resolveURN: resolveURN!,
        resolveID: resolveID,
        resolvers: resolvers,
        serializedProps: serializedProps,
        parentURN: parentURN,
        providerRef: providerRef,
        allDirectDependencyURNs: allDirectDependencyURNs,
        propertyToDirectDependencyURNs: propertyToDirectDependencyURNs,
    };
}

function addAll<T>(to: Set<T>, from: Set<T>) {
    for (const val of from) {
        to.add(val);
    }
}

async function getAllTransitivelyReferencedCustomResourceURNs(resources: Set<Resource>) {
    // Go through 'resources', but transitively walk through **Component** resources, collecting any
    // of their child resources.  This way, a Component acts as an aggregation really of all the
    // reachable custom resources it parents.  This walking will transitively walk through other
    // child ComponentResources, but will stop when it hits custom resources.  in other words, if we
    // had:
    //
    //              Comp1
    //              /   \
    //          Cust1   Comp2
    //                  /   \
    //              Cust2   Cust3
    //              /
    //          Cust4
    //
    // Then the transitively reachable custom resources of Comp1 will be [Cust1, Cust2, Cust3]. It
    // will *not* include `Cust4`.

    // To do this, first we just get the transitively reachable set of resources (not diving
    // into custom resources).  In the above picture, if we start with 'Comp1', this will be
    // [Comp1, Cust1, Comp2, Cust2, Cust3]
    const transitivelyReachableResources = getTransitivelyReferencedChildResourcesOfComponentResources(resources);

    const transitivelyReachableCustomResources =  [...transitivelyReachableResources].filter(r => CustomResource.isInstance(r));
    const promises = transitivelyReachableCustomResources.map(r => r.urn.promise());
    const urns = await Promise.all(promises);
    return new Set<string>(urns);
}

/**
 * Recursively walk the resources passed in, returning them and all resources reachable from
 * [Resource.__childResources] through any **Component** resources we encounter.
 */
function getTransitivelyReferencedChildResourcesOfComponentResources(resources: Set<Resource>) {
    // Recursively walk the dependent resources through their children, adding them to the result set.
    const result = new Set<Resource>();
    addTransitivelyReferencedChildResourcesOfComponentResources(resources, result);
    return result;
}

function addTransitivelyReferencedChildResourcesOfComponentResources(resources: Set<Resource> | undefined, result: Set<Resource>) {
    if (resources) {
        for (const resource of resources) {
            if (!result.has(resource)) {
                result.add(resource);

                if (ComponentResource.isInstance(resource)) {
                    addTransitivelyReferencedChildResourcesOfComponentResources(resource.__childResources, result);
                }
            }
        }
    }
}

/**
 * Gathers explicit dependent Resources from a list of Resources (possibly Promises and/or Outputs).
 */
async function gatherExplicitDependencies(
    dependsOn: Input<Input<Resource>[]> | Input<Resource> | undefined): Promise<Resource[]> {

    if (dependsOn) {
        if (Array.isArray(dependsOn)) {
            const dos: Resource[] = [];
            for (const d of dependsOn) {
                dos.push(...(await gatherExplicitDependencies(d)));
            }
            return dos;
        } else if (dependsOn instanceof Promise) {
            return gatherExplicitDependencies(await dependsOn);
        } else if (Output.isInstance(dependsOn)) {
            // Recursively gather dependencies, await the promise, and append the output's dependencies.
            const dos = (dependsOn as Output<Input<Resource>[] | Input<Resource>>).apply(v => gatherExplicitDependencies(v));
            const urns = await dos.promise();
            const implicits = await gatherExplicitDependencies([...dos.resources()]);
            return urns.concat(implicits);
        } else {
            if (!Resource.isInstance(dependsOn)) {
                throw new Error("'dependsOn' was passed a value that was not a Resource.");
            }

            return [dependsOn];
        }
    }

    return [];
}

/**
 * Finishes a resource creation RPC operation by resolving its outputs to the resulting RPC payload.
 */
async function resolveOutputs(res: Resource, t: string, name: string,
                              props: Inputs, outputs: any, resolvers: OutputResolvers): Promise<void> {
    // Produce a combined set of property states, starting with inputs and then applying
    // outputs.  If the same property exists in the inputs and outputs states, the output wins.
    const allProps: Record<string, any> = {};
    if (outputs) {
        Object.assign(allProps, deserializeProperties(outputs));
    }

    const label = `resource:${name}[${t}]#...`;
    for (const key of Object.keys(props)) {
        if (!allProps.hasOwnProperty(key)) {
            // input prop the engine didn't give us a final value for.  Just use the value passed into the resource
            // after round-tripping it through serialization. We do the round-tripping primarily s.t. we ensure that
            // Output values are handled properly w.r.t. unknowns.
            const inputProp = await serializeProperty(label, props[key], new Set());
            if (inputProp === undefined) {
                continue;
            }
            allProps[key] = deserializeProperty(inputProp);
        }
    }

    resolveProperties(res, resolvers, t, name, allProps);
}

/**
 * registerResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
 */
export function registerResourceOutputs(res: Resource, outputs: Inputs | Promise<Inputs> | Output<Inputs>) {
    // Now run the operation. Note that we explicitly do not serialize output registration with
    // respect to other resource operations, as outputs may depend on properties of other resources
    // that will not resolve until later turns. This would create a circular promise chain that can
    // never resolve.
    const opLabel = `monitor.registerResourceOutputs(...)`;
    runAsyncResourceOp(opLabel, async () => {
        // The registration could very well still be taking place, so we will need to wait for its URN.
        // Additionally, the output properties might have come from other resources, so we must await those too.
        const urn = await res.urn.promise();
        const resolved = await serializeProperties(opLabel, { outputs });
        const outputsObj = gstruct.Struct.fromJavaScript(resolved.outputs);
        log.debug(`RegisterResourceOutputs RPC prepared: urn=${urn}` +
            (excessiveDebugOutput ? `, outputs=${JSON.stringify(outputsObj)}` : ``));

        // Fetch the monitor and make an RPC request.
        const monitor = getMonitor();
        if (monitor) {
            const req = new resproto.RegisterResourceOutputsRequest();
            req.setUrn(urn);
            req.setOutputs(outputsObj);

            const label = `monitor.registerResourceOutputs(${urn}, ...)`;
            await debuggablePromise(new Promise((resolve, reject) =>
                (monitor as any).registerResourceOutputs(req, (err: grpc.ServiceError, innerResponse: any) => {
                    log.debug(`RegisterResourceOutputs RPC finished: urn=${urn}; `+
                        `err: ${err}, resp: ${innerResponse}`);
                    if (err) {
                        // If the monitor is unavailable, it is in the process of shutting down or has already
                        // shut down. Don't emit an error and don't do any more RPCs, just exit.
                        if (err.code === grpc.status.UNAVAILABLE) {
                            log.debug("Resource monitor is terminating");
                            process.exit(0);
                        }

                        log.error(`Failed to end new resource registration '${urn}': ${err.stack}`);
                        reject(err);
                    }
                    else {
                        resolve();
                    }
                })), label);
            }
    }, false);
}

/**
 * resourceChain is used to serialize all resource requests.  If we don't do this, all resource operations will be
 * entirely asynchronous, meaning the dataflow graph that results will determine ordering of operations.  This
 * causes problems with some resource providers, so for now we will serialize all of them.  The issue
 * pulumi/pulumi#335 tracks coming up with a long-term solution here.
 */
let resourceChain: Promise<void> = Promise.resolve();
let resourceChainLabel: string | undefined = undefined;

// runAsyncResourceOp runs an asynchronous resource operation, possibly serializing it as necessary.
function runAsyncResourceOp(label: string, callback: () => Promise<void>, serial?: boolean): void {
    // Serialize the invocation if necessary.
    if (serial === undefined) {
        serial = serialize();
    }
    const resourceOp: Promise<void> = debuggablePromise(resourceChain.then(async () => {
        if (serial) {
            resourceChainLabel = label;
            log.debug(`Resource RPC serialization requested: ${label} is current`);
        }
        return callback();
    }), label + "-initial");

    // Ensure the process won't exit until this RPC call finishes and resolve it when appropriate.
    const done: () => void = rpcKeepAlive();
    const finalOp: Promise<void> = debuggablePromise(
        resourceOp.then(() => { done(); }, () => { done(); }),
        label + "-final");

    // Set up another promise that propagates the error, if any, so that it triggers unhandled rejection logic.
    resourceOp.catch((err) => Promise.reject(err));

    // If serialization is requested, wait for the prior resource operation to finish before we proceed, serializing
    // them, and make this the current resource operation so that everybody piles up on it.
    if (serial) {
        resourceChain = finalOp;
        if (resourceChainLabel) {
            log.debug(`Resource RPC serialization requested: ${label} is behind ${resourceChainLabel}`);
        }
    }
}
