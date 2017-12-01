// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { Computed, ComputedValue, ComputedValues, ID, Resource, URN } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { PropertyTransfer, resolveProperties, transferProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

const resproto = require("../proto/resource_pb.js");

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
 * URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, custom: boolean,
                                 props: ComputedValues | undefined, parent: Resource | undefined,
                                 dependsOn: Resource[] | undefined): void {
    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}` +
        (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    const preError: Error = new Error(`Resource '${name}' [${t}] could not be registered`);

    // Simply initialize the URN property and get prepared to resolve it later on.
    let resolveURN: ((urn: URN | undefined) => void) | undefined;
    (res as any).urn = debuggablePromise(
        new Promise<URN | undefined>((resolve) => { resolveURN = resolve; }),
        `resolveURN(${label})`,
    );

    // If a custom resource, make room for the ID property.
    let resolveID: ((v: ID | undefined) => void) | undefined;
    if (custom) {
        (res as any).id = debuggablePromise(
            new Promise<ID | undefined>((resolve) => { resolveID = resolve; }),
            `resolveID(${label})`,
        );
    }

    // Now "transfer" all input properties; this simply awaits any promises and resolves when they all do.
    const transfer: Promise<PropertyTransfer> = debuggablePromise(
        transferProperties(res, label, props, dependsOn), `transferProperties(${label})`);

    // Now run the operation, serializing the invocation if necessary.
    const opLabel = `monitor.registerResource(${label})`;
    runAsyncResourceOp(opLabel, preError, async () => {
        // During a real deployment, the transfer operation may take some time to settle (we may need to wait on
        // other in-flight operations.  As a result, we can't launch the RPC request until they are done.  At the same
        // time, we want to give the illusion of non-blocking code, so we return immediately.
        const result: PropertyTransfer = await transfer;

        // Make sure to assign all of these properties.
        let urn: URN | undefined = undefined;
        let id: ID | undefined = undefined;
        let propsStruct: any | undefined = undefined;
        let stable: boolean = false;
        let stables: Set<string> | undefined = undefined;
        try {
            const obj: any = result.obj;
            log.debug(`RegisterResource RPC prepared: t=${t}, name=${name}` +
                (excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``));

            // Fetch the monitor and make an RPC request.
            const monitor: any = getMonitor();
            if (monitor) {
                let parentURN: URN | undefined;
                if (parent) {
                    parentURN = await parent.urn;
                }

                const req = new resproto.RegisterResourceRequest();
                req.setType(t);
                req.setName(name);
                req.setParent(parentURN);
                req.setCustom(custom);
                req.setObject(obj);

                const resp: any = await debuggablePromise(new Promise((resolve, reject) => {
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
                    });
                }), opLabel);

                urn = resp.getUrn();
                id = resp.getId();
                propsStruct = resp.getObject();
                stable = resp.getStable();

                const stablesList: string[] | undefined = resp.getStablesList();
                if (stablesList) {
                    stables = new Set<string>();
                    for (const sta of stablesList) {
                        stables.add(sta);
                    }
                }
            }
            else {
                // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
                log.warn(`Not sending RPC to monitor -- it doesn't exist: t=${t}, name=${name}`);
            }
        }
        finally {
            // Always make sure to resolve the URN property, even if it is undefined due to a missing monitor.
            resolveURN!(urn);

            // If an ID is present, then it's safe to say it's final, because the resource planner wouldn't hand
            // it back to us otherwise (e.g., if the resource was being replaced, it would be missing).  If it isn't
            // available, ensure the ID gets resolved, just resolve it to undefined (indicating it isn't known).
            if (resolveID) {
                resolveID!(id || undefined);
            }

            // Propagate any other properties that were given to us as outputs.
            resolveProperties(res, result, t, name, props, propsStruct, stable, stables);
        }
    });
}

/**
 * registerResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
 */
export function registerResourceOutputs(res: Resource, outputs: ComputedValues) {
    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    const preError: Error = new Error(`Resource outputs could not be registered`);

    // Produce the "extra" values, if any, that we'll use in the RPC call.
    const transfer: Promise<PropertyTransfer> = debuggablePromise(
        transferProperties(undefined, `completeResource`, outputs, undefined));

    // Now run the operation, serializing the invocation if necessary.
    const opLabel = `monitor.registerResourceOutputs(...)`;
    runAsyncResourceOp(opLabel, preError, async () => {
        // The registration could very well still be taking place, so we will need to wait for its URN.  Additionally,
        // the output properties might have come from other resources, so we must await those too.
        const urn: URN = await res.urn;
        const outputsObj: any = (await transfer).obj;
        log.debug(`RegisterResourceOutputs RPC prepared: urn=${urn}` +
            (excessiveDebugOutput ? `, outputs=${JSON.stringify(outputsObj)}` : ``));

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();
        if (monitor) {
            const req = new resproto.RegisterResourceOutputsRequest();
            req.setUrn(urn);
            req.setOutputs(outputsObj);

            await debuggablePromise(new Promise((resolve, reject) => {
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
                });
            }), opLabel);
        }
        else {
            // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
            log.warn(`Not sending RPC to monitor -- it doesn't exist: urn=${urn}`);
        }
    });
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
function runAsyncResourceOp(label: string, rootError: Error, callback: () => Promise<void>): void {
    // Serialize the invocation if necessary.
    const serial: boolean = serialize();
    const resourceOp: Promise<void> = debuggablePromise(resourceChain.then(async () => {
        if (serial) {
            resourceChainLabel = label;
            log.debug(`Resource RPC serialization requested: ${label} is current`);
        }
        return callback();
    }));

    // If any errors make it this far, ensure we log them.
    const finalOp: Promise<void> = debuggablePromise(resourceOp.catch((err: Error) => {
        // At this point, we've gone fully asynchronous, and the stack is missing.  To make it easier
        // to debug which resource this came from, we will emit the original stack trace too.
        log.error(errorString(err));
        log.error(`Resource RPC for '${label}' failed: ${errorString(rootError)}`);
    }));

    // Ensure the process won't exit until this registerResource call finishes and resolve it when appropriate.
    const done: () => void = rpcKeepAlive();
    finalOp.then(() => { done(); }, () => { done(); });

    // If serialization is requested, wait for the prior resource operation to finish before we proceed, serializing
    // them, and make this the current resource operation so that everybody piles up on it.
    if (serial) {
        resourceChain = finalOp;
        if (resourceChainLabel) {
            log.debug(`Resource RPC serialization requested: ${label} is behind ${resourceChainLabel}`);
        }
    }
}
