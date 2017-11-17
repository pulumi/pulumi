// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { Computed, ComputedValue, ComputedValues, ID, Resource, URN } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { PropertyTransfer, resolveProperties, transferProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

const resproto = require("../proto/resource_pb.js");

/**
 * resourceChain is used to serialize all resource requests.  If we don't do this, all resource operations will be
 * entirely asynchronous, meaning the dataflow graph that results will determine ordering of operations.  This
 * causes problems with some resource providers, so for now we will serialize all of them.  The issue
 * pulumi/pulumi#335 tracks coming up with a long-term solution here.
 */
let resourceChain: Promise<void> = Promise.resolve();
let resourceChainLabel: string | undefined = undefined;

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated URN
 * and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, custom: boolean,
                                 props: ComputedValues | undefined, parent: Resource | undefined,
                                 dependsOn: Resource[] | undefined): void {
    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}` +
        (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    const createError: Error = new Error(`Resouce '${name}' [${t}] could not be created`);

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

    // Ensure we depend on any children plus any explicit dependsOns.
    const allDependsOn: Resource[] = [];
    if (parent) {
        allDependsOn.push(parent);
    }
    if (dependsOn) {
        allDependsOn.push(...dependsOn);
    }

    // Now "transfer" all input properties; this simply awaits any promises and resolves when they all do.
    const transfer: Promise<PropertyTransfer> = debuggablePromise(
        transferProperties(res, label, props, allDependsOn), `transferProperties(${label})`);

    // Serialize the invocation if necessary.
    const resourceOp: Promise<void> = debuggablePromise(resourceChain.then(async () => {
        if (serialize()) {
            resourceChainLabel = `${name} [${t}]`;
            log.debug(`Resource serialization requested: ${resourceChainLabel} is current`);
        }

        // Make sure to propagate these no matter what.
        let urn: URN | undefined = undefined;
        let id: ID | undefined = undefined;
        let propsStruct: any | undefined = undefined;
        let stable: boolean = false;
        let stables: Set<string> | undefined = undefined;

        // During a real deployment, the transfer operation may take some time to settle (we may need to wait on
        // other in-flight operations.  As a result, we can't launch the RPC request until they are done.  At the same
        // time, we want to give the illusion of non-blocking code, so we return immediately.
        const result: PropertyTransfer = await transfer;
        try {
            const obj: any = result.obj;
            log.debug(`Resource RPC prepared: t=${t}, name=${name}` +
                (excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``));

            // Fetch the monitor and make an RPC request.
            const monitor: any = getMonitor();
            if (monitor) {
                let parentURN: URN | undefined;
                if (parent) {
                    parentURN = await parent.urn;
                }

                const req = new resproto.NewResourceRequest();
                req.setType(t);
                req.setName(name);
                req.setParent(parentURN);
                req.setCustom(custom);
                req.setObject(obj);

                const resp: any = await debuggablePromise(new Promise((resolve, reject) => {
                    monitor.newResource(req, (err: Error, innerResponse: any) => {
                        log.debug(`Resource RPC finished: t=${t}, name=${name}; err: ${err}, resp: ${innerResponse}`);
                        if (err) {
                            log.error(`Failed to register new resource '${name}' [${t}]: ${err}`);
                            reject(err);
                        }
                        else {
                            resolve(innerResponse);
                        }
                    });
                }), `monitor.newResource(${label})`);

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
            // If an ID is present, then it's safe to say it's final, because the resource planner wouldn't hand
            // it back to us otherwise (e.g., if the resource was being replaced, it would be missing).  If it isn't
            // available, ensure the ID gets resolved, just resolve it to undefined (indicating it isn't known).
            if (resolveID) {
                resolveID(id || undefined);
            }

            // Propagate any other properties that were given to us as outputs.
            resolveProperties(res, result, t, name, props, propsStruct, stable, stables);

            // Finally, the resolution will always have a valid URN, even during planning; set it.
            resolveURN!(urn);
        }
    }));

    // If any errors make it this far, ensure we log them.
    const finalOp: Promise<void> = debuggablePromise(resourceOp.catch((err: Error) => {
        // At this point, we've gone fully asynchronous, and the stack is missing.  To make it easier
        // to debug which resource this came from, we will emit the original stack trace too.
        log.error(errorString(createError));
        log.error(`Failed to create resource '${name}' [${t}]: ${errorString(err)}`);
    }));

    // Ensure the process won't exit until this registerResource call finishes and resolve it when appropriate.
    const done: () => void = rpcKeepAlive();
    finalOp.then(() => { done(); }, () => { done(); });

    // If serialization is requested, wait for the prior resource operation to finish before we proceed, serializing
    // them, and make this the current resource operation so that everybody piles up on it.
    if (serialize()) {
        resourceChain = finalOp;
        if (resourceChainLabel) {
            log.debug(`Resource serialization requested: ${name} [${t}] is behind ${resourceChainLabel}`);
        }
    }
}
