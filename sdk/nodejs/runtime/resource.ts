// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { Computed, ComputedValue, ComputedValues, ID, Resource, ResourceOptions, URN } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { resolveProperties, serializeProperties, transferProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const resproto = require("../proto/resource_pb.js");

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
 * URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, custom: boolean,
                                 props: ComputedValues, opts: ResourceOptions): void {

    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}` +
        (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    // Simply initialize the URN property and get prepared to resolve it later on.
    let resolveURN: (urn: URN | undefined) => void;
    (res as any).urn = debuggablePromise(
        new Promise<URN | undefined>(resolve => resolveURN = resolve),
        `resolveURN(${label})`);

    // If a custom resource, make room for the ID property.
    let resolveID: ((v: ID | undefined) => void) | undefined;
    if (custom) {
        (res as any).id = debuggablePromise(
            new Promise<ID | undefined>(resolve => resolveID = resolve),
            `resolveID(${label})`);
    }

    // Now "transfer" all input properties into unresolved Promises on res.  This way,
    // this resource will look like it has all its output properties to anyone it is
    // passed to.  However, those promises won't actually resolve until the registerResource
    // RPC returns
    const resolvers = transferProperties(res, label, props);

    // Now run the operation, serializing the invocation if necessary.
    const opLabel = `monitor.registerResource(${label})`;
    runAsyncResourceOp(opLabel, async () => {
        // Before we can proceed, all our dependencies must be finished.
        const dependsOn = opts.dependsOn || [];
        await debuggablePromise(Promise.all(dependsOn.map(d => d.urn)), `dependsOn(${label})`);

        // Make sure to assign all of these properties.
        let urn: URN | undefined;
        let id: ID | undefined;
        let propsStruct: any | undefined;
        let stable = false;
        let stables = new Set<string>();
        try {
            const obj = gstruct.Struct.fromJavaScript(await serializeProperties(label, props));
            log.debug(`RegisterResource RPC prepared: t=${t}, name=${name}` +
                (excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``));

            // Fetch the monitor and make an RPC request.
            const monitor: any = getMonitor();
            if (!monitor) {
                // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
                log.warn(`Not sending RPC to monitor -- it doesn't exist: t=${t}, name=${name}`);
                return;
            }

            let parentURN: URN | undefined;
            if (opts.parent) {
                parentURN = await opts.parent.urn;
            }

            const req = new resproto.RegisterResourceRequest();
            req.setType(t);
            req.setName(name);
            req.setParent(parentURN);
            req.setCustom(custom);
            req.setObject(obj);
            req.setProtect(opts.protect);

            const resp: any = await debuggablePromise(new Promise((resolve, reject) =>
                monitor.registerResource(req, (err: Error, innerResponse: any) => {
                    log.debug(`RegisterResource RPC finished: t=${t}, name=${name}; ` +
                        `err: ${err}, resp: ${innerResponse}`);
                    if (err) {
                        log.error(`Failed to register new resource '${name}' [${t}]: ${err.stack}`);
                        reject(err);
                    }
                    else {
                        resolve(innerResponse);
                    }
                })), opLabel);

            urn = resp.getUrn();
            id = resp.getId();
            propsStruct = resp.getObject();
            stable = resp.getStable();

            const stablesList: string[] | undefined = resp.getStablesList();
            stables = new Set<string>(stablesList);
        }
        finally {
            // Always make sure to resolve the URN property, even if it is undefined due to a
            // missing monitor.
            resolveURN(urn);

            // If an ID is present, then it's safe to say it's final, because the resource planner
            // wouldn't hand it back to us otherwise (e.g., if the resource was being replaced, it
            // would be missing).  If it isn't available, ensure the ID gets resolved, just resolve
            // it to undefined (indicating it isn't known).
            if (resolveID) {
                resolveID(id);
            }

            // Propagate any other properties that were given to us as outputs.
            resolveProperties(res, resolvers, t, name, props, propsStruct, stable, stables);
        }
    });
}

/**
 * registerResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
 */
export function registerResourceOutputs(res: Resource, outputs: ComputedValues) {
    // Now run the operation. Note that we explicitly do not serialize output registration with
    // respect to other resource operations, as outputs may depend on properties of other resources
    // that will not resolve until later turns. This would create a circular promise chain that can
    // never resolve.
    const opLabel = `monitor.registerResourceOutputs(...)`;
    runAsyncResourceOp(opLabel, async () => {
        // The registration could very well still be taking place, so we will need to wait for its URN.  Additionally,
        // the output properties might have come from other resources, so we must await those too.
        const urn: URN = await res.urn;
        const outputsObj = gstruct.Struct.fromJavaScript(
            await serializeProperties(`completeResource`, outputs));
        log.debug(`RegisterResourceOutputs RPC prepared: urn=${urn}` +
            (excessiveDebugOutput ? `, outputs=${JSON.stringify(outputsObj)}` : ``));

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();
        if (!monitor) {
            // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
            log.warn(`Not sending RPC to monitor -- it doesn't exist: urn=${urn}`);
            return;
        }

        const req = new resproto.RegisterResourceOutputsRequest();
        req.setUrn(urn);
        req.setOutputs(outputsObj);

        await debuggablePromise(new Promise((resolve, reject) =>
            monitor.registerResourceOutputs(req, (err: Error, innerResponse: any) => {
                log.debug(`RegisterResourceOutputs RPC finished: urn=${urn}; `+
                    `err: ${err}, resp: ${innerResponse}`);
                if (err) {
                    log.error(`Failed to end new resource registration '${urn}': ${err.stack}`);
                    reject(err);
                }
                else {
                    resolve();
                }
            })), opLabel);
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
    }));

    // Ensure the process won't exit until this RPC call finishes and resolve it when appropriate.
    const done: () => void = rpcKeepAlive();
    const finalOp: Promise<void> = debuggablePromise(resourceOp.then(() => { done(); }, () => { done(); }));

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
