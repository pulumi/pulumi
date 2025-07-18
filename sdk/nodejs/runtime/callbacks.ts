// Copyright 2016-2024, Pulumi Corporation.
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

import * as grpc from "@grpc/grpc-js";
import { randomUUID } from "crypto";
import * as jspb from "google-protobuf";
import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as log from "../log";
import { output } from "../output";
import * as callrpc from "../proto/callback_grpc_pb";
import * as callproto from "../proto/callback_pb";
import { Callback, CallbackInvokeRequest, CallbackInvokeResponse } from "../proto/callback_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import * as resproto from "../proto/resource_pb";
import {
    Alias,
    ComponentResourceOptions,
    CustomResourceOptions,
    DependencyProviderResource,
    DependencyResource,
    ProviderResource,
    Resource,
    ResourceHook,
    ResourceOptions,
    ResourceTransform,
    ResourceTransformArgs,
    URN,
    rootStackResource,
} from "../resource";
import { InvokeOptions, InvokeTransform, InvokeTransformArgs } from "../invoke";

import { hookBindingFromProto, mapAliasesForRequest, prepareHooks } from "./resource";
import { deserializeProperties, serializeProperties, unknownValue } from "./rpc";
import { debuggablePromise } from "./debuggable";
import { rpcKeepAlive } from "./settings";

/**
 * Raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
 *
 * @internal
 */
const maxRPCMessageSize: number = 1024 * 1024 * 400;

type CallbackFunction = (args: Uint8Array) => Promise<jspb.Message>;

export interface ICallbackServer {
    registerTransform(callback: ResourceTransform): Promise<callproto.Callback>;
    registerStackTransform(callback: ResourceTransform): void;
    registerStackInvokeTransform(callback: InvokeTransform): void;
    registerStackInvokeTransformAsync(callback: InvokeTransform): Promise<callproto.Callback>;
    registerResourceHook(hook: ResourceHook): Promise<void>;
    shutdown(): void;
    // Wait for any pendind registerStackTransform calls to complete.
    awaitStackRegistrations(): Promise<void>;
}

export class CallbackServer implements ICallbackServer {
    private readonly _callbacks = new Map<string, CallbackFunction>();
    private readonly _monitor: resrpc.IResourceMonitorClient;
    private readonly _server: grpc.Server;
    private readonly _target: Promise<string>;
    private _pendingRegistrations: number = 0;
    private _awaitQueue: ((reason?: any) => void)[] = [];

    constructor(monitor: resrpc.IResourceMonitorClient) {
        this._monitor = monitor;

        this._server = new grpc.Server({
            "grpc.max_receive_message_length": maxRPCMessageSize,
        });

        const implementation: callrpc.ICallbacksServer = {
            invoke: this.invoke.bind(this),
        };
        this._server.addService(callrpc.CallbacksService, implementation);

        const self = this;
        this._target = new Promise<string>((resolve, reject) => {
            self._server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, port) => {
                if (err !== null) {
                    reject(err);
                    return;
                }

                // The server takes a while to _actually_ startup so we need to keep trying to send an invoke
                // to ourselves before we resolve the address to tell the engine about it.
                const target = `127.0.0.1:${port}`;

                const client = new callrpc.CallbacksClient(target, grpc.credentials.createInsecure());

                const connect = () => {
                    client.invoke(new CallbackInvokeRequest(), (error, _) => {
                        if (error?.code === grpc.status.UNAVAILABLE) {
                            setTimeout(connect, 1000);
                            return;
                        }
                        // The expected error given we didn't give a token to the invoke.
                        if (error?.details === "callback not found: ") {
                            resolve(target);
                            return;
                        }
                        reject(error);
                    });
                };
                connect();
            });
        });
    }

    awaitStackRegistrations(): Promise<void> {
        if (this._pendingRegistrations === 0) {
            return Promise.resolve();
        }
        return new Promise<void>((resolve, reject) => {
            this._awaitQueue.push((reason?: any) => {
                if (reason !== undefined) {
                    reject(reason);
                } else {
                    resolve();
                }
            });
        });
    }

    shutdown(): void {
        this._server.forceShutdown();
    }

    private async invoke(
        call: grpc.ServerUnaryCall<CallbackInvokeRequest, CallbackInvokeResponse>,
        callback: grpc.sendUnaryData<CallbackInvokeResponse>,
    ) {
        const req = call.request;

        const cb = this._callbacks.get(req.getToken());
        if (cb === undefined) {
            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.INVALID_ARGUMENT);
            err.withDetails("callback not found: " + req.getToken());
            callback(err.build());
            return;
        }

        try {
            const response = await cb(req.getRequest_asU8());
            const resp = new CallbackInvokeResponse();
            resp.setResponse(response.serializeBinary());
            callback(null, resp);
        } catch (e) {
            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.UNKNOWN);
            if (e instanceof Error) {
                err.withDetails(e.message);
            } else {
                err.withDetails(JSON.stringify(e));
            }
            callback(err.build());
        }
    }

    async registerTransform(transform: ResourceTransform): Promise<callproto.Callback> {
        const cb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            const request = resproto.TransformRequest.deserializeBinary(bytes);

            let opts = request.getOptions() || new resproto.TransformResourceOptions();

            let ropts: ResourceOptions;
            if (request.getCustom()) {
                ropts = {
                    deleteBeforeReplace: opts.getDeleteBeforeReplace(),
                    additionalSecretOutputs: opts.getAdditionalSecretOutputsList(),
                } as CustomResourceOptions;
            } else {
                const providers: Record<string, ProviderResource> = {};
                for (const [key, value] of opts.getProvidersMap().entries()) {
                    providers[key] = new DependencyProviderResource(value);
                }
                ropts = {
                    providers: providers,
                } as ComponentResourceOptions;
            }

            ropts.aliases = opts.getAliasesList().map((alias): string | Alias => {
                if (alias.hasUrn()) {
                    return alias.getUrn();
                } else {
                    const spec = alias.getSpec();
                    if (spec === undefined) {
                        throw new Error("alias must have either a urn or a spec");
                    }
                    const nodeAlias: Alias = {
                        name: spec.getName(),
                        type: spec.getType(),
                        project: spec.getProject(),
                        stack: spec.getStack(),
                        parent: spec.getParenturn() !== "" ? new DependencyResource(spec.getParenturn()) : undefined,
                    };

                    if (spec.getNoparent()) {
                        nodeAlias.parent = rootStackResource;
                    }

                    return nodeAlias;
                }
            });
            const timeouts = opts.getCustomTimeouts();
            if (timeouts !== undefined) {
                ropts.customTimeouts = {
                    create: timeouts.getCreate(),
                    update: timeouts.getUpdate(),
                    delete: timeouts.getDelete(),
                };
            }
            ropts.hooks = hookBindingFromProto(opts.getHooks());
            ropts.deletedWith =
                opts.getDeletedWith() !== "" ? new DependencyResource(opts.getDeletedWith()) : undefined;
            ropts.dependsOn = opts.getDependsOnList().map((dep) => new DependencyResource(dep));
            ropts.ignoreChanges = opts.getIgnoreChangesList();
            ropts.parent = request.getParent() !== "" ? new DependencyResource(request.getParent()) : undefined;
            ropts.pluginDownloadURL = opts.getPluginDownloadUrl() !== "" ? opts.getPluginDownloadUrl() : undefined;
            ropts.protect = opts.getProtect();
            ropts.provider = opts.getProvider() !== "" ? new DependencyProviderResource(opts.getProvider()) : undefined;
            ropts.replaceOnChanges = opts.getReplaceOnChangesList();
            ropts.retainOnDelete = opts.getRetainOnDelete();
            ropts.version = opts.getVersion() !== "" ? opts.getVersion() : undefined;

            const props = request.getProperties();

            const args: ResourceTransformArgs = {
                custom: request.getCustom(),
                type: request.getType(),
                name: request.getName(),
                props: props === undefined ? {} : deserializeProperties(props),
                opts: ropts,
            };

            const result = await transform(args);

            const response = new resproto.TransformResponse();
            if (result === undefined) {
                response.setProperties(request.getProperties());
                response.setOptions(request.getOptions());
            } else {
                const mprops = await serializeProperties("props", result.props);
                response.setProperties(gstruct.Struct.fromJavaScript(mprops));

                // Copy the options over.
                if (result.opts !== undefined) {
                    opts = new resproto.TransformResourceOptions();

                    if (result.opts.aliases !== undefined) {
                        const aliases = [];
                        const uniqueAliases = new Set<Alias | URN>();
                        for (const alias of result.opts.aliases || []) {
                            const aliasVal = await output(alias).promise();
                            if (!uniqueAliases.has(aliasVal)) {
                                uniqueAliases.add(aliasVal);
                                aliases.push(aliasVal);
                            }
                        }

                        opts.setAliasesList(await mapAliasesForRequest(aliases, request.getParent()));
                    }
                    if (result.opts.customTimeouts !== undefined) {
                        const customTimeouts = new resproto.RegisterResourceRequest.CustomTimeouts();
                        if (result.opts.customTimeouts.create !== undefined) {
                            customTimeouts.setCreate(result.opts.customTimeouts.create);
                        }
                        if (result.opts.customTimeouts.update !== undefined) {
                            customTimeouts.setUpdate(result.opts.customTimeouts.update);
                        }
                        if (result.opts.customTimeouts.delete !== undefined) {
                            customTimeouts.setDelete(result.opts.customTimeouts.delete);
                        }
                        opts.setCustomTimeouts(customTimeouts);
                    }
                    if (result.opts.deletedWith !== undefined) {
                        opts.setDeletedWith(await result.opts.deletedWith.urn.promise());
                    }
                    if (result.opts.dependsOn !== undefined) {
                        const resolvedDeps = await output(result.opts.dependsOn).promise();
                        const deps = [];
                        if (Resource.isInstance(resolvedDeps)) {
                            deps.push(await resolvedDeps.urn.promise());
                        } else {
                            for (const dep of resolvedDeps) {
                                deps.push(await dep.urn.promise());
                            }
                        }
                        opts.setDependsOnList(deps);
                    }
                    if (result.opts.ignoreChanges !== undefined) {
                        opts.setIgnoreChangesList(result.opts.ignoreChanges);
                    }
                    if (result.opts.pluginDownloadURL !== undefined) {
                        opts.setPluginDownloadUrl(result.opts.pluginDownloadURL);
                    }
                    if (result.opts.protect !== undefined) {
                        opts.setProtect(result.opts.protect);
                    }
                    if (result.opts.provider !== undefined) {
                        const providerURN = await result.opts.provider.urn.promise();
                        const providerID = (await result.opts.provider.id.promise()) || unknownValue;
                        opts.setProvider(`${providerURN}::${providerID}`);
                    }
                    if (result.opts.replaceOnChanges !== undefined) {
                        opts.setReplaceOnChangesList(result.opts.replaceOnChanges);
                    }
                    if (result.opts.retainOnDelete !== undefined) {
                        opts.setRetainOnDelete(result.opts.retainOnDelete);
                    }
                    if (result.opts.version !== undefined) {
                        opts.setVersion(result.opts.version);
                    }
                    if (result.opts.hooks !== undefined) {
                        opts.setHooks(await prepareHooks(result.opts.hooks, request.getName()));
                    }

                    if (request.getCustom()) {
                        const copts = result.opts as CustomResourceOptions;
                        if (copts.deleteBeforeReplace !== undefined) {
                            opts.setDeleteBeforeReplace(copts.deleteBeforeReplace);
                        }
                        if (copts.additionalSecretOutputs !== undefined) {
                            opts.setAdditionalSecretOutputsList(copts.additionalSecretOutputs);
                        }
                    } else {
                        const copts = result.opts as ComponentResourceOptions;

                        if (copts.providers !== undefined) {
                            const providers = opts.getProvidersMap();

                            if (copts.providers && !Array.isArray(copts.providers)) {
                                for (const k in copts.providers) {
                                    if (Object.prototype.hasOwnProperty.call(copts.providers, k)) {
                                        const v = copts.providers[k];
                                        if (k !== v.getPackage()) {
                                            const message = `provider resource map where key ${k} doesn't match provider ${v.getPackage()}`;
                                            log.warn(message);
                                        }
                                    }
                                }
                            }
                            const provs = Object.values(copts.providers);
                            for (const prov of provs) {
                                const providerURN = await prov.urn.promise();
                                const providerID = (await prov.id.promise()) || unknownValue;
                                providers.set(prov.getPackage(), `${providerURN}::${providerID}`);
                            }
                            opts.clearProvidersMap();
                        }
                    }
                }
                response.setOptions(opts);
            }

            return response;
        };
        const tryCb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            try {
                return await cb(bytes);
            } catch (e) {
                throw new Error(`transform failed: ${e}`);
            }
        };
        const uuid = randomUUID();
        this._callbacks.set(uuid, tryCb);
        const req = new Callback();
        req.setToken(uuid);
        req.setTarget(await this._target);
        return req;
    }

    registerStackTransform(transform: ResourceTransform): void {
        this._pendingRegistrations++;

        this.registerTransform(transform)
            .then(
                (req) => {
                    return new Promise((resolve, reject) => {
                        this._monitor.registerStackTransform(req, (err, _) => {
                            if (err !== null) {
                                // Remove this from the list of callbacks given we didn't manage to actually register it.
                                this._callbacks.delete(req.getToken());
                                reject();
                            } else {
                                resolve();
                            }
                        });
                    });
                },
                (err) => log.error(`failed to register stack transform: ${err}`),
            )
            .finally(() => {
                this._pendingRegistrations--;
                if (this._pendingRegistrations === 0) {
                    const queue = this._awaitQueue;
                    this._awaitQueue = [];
                    for (const waiter of queue) {
                        waiter();
                    }
                }
            });
    }

    async registerStackInvokeTransformAsync(transform: InvokeTransform): Promise<callproto.Callback> {
        const cb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            const request = resproto.TransformInvokeRequest.deserializeBinary(bytes);

            let opts = request.getOptions() || new resproto.TransformInvokeOptions();

            const ropts: InvokeOptions = {};
            ropts.pluginDownloadURL = opts.getPluginDownloadUrl() !== "" ? opts.getPluginDownloadUrl() : undefined;
            ropts.provider = opts.getProvider() !== "" ? new DependencyProviderResource(opts.getProvider()) : undefined;
            ropts.version = opts.getVersion() !== "" ? opts.getVersion() : undefined;

            const invokeArgs = request.getArgs();

            const args: InvokeTransformArgs = {
                token: request.getToken(),
                args: invokeArgs === undefined ? {} : deserializeProperties(invokeArgs),
                opts: ropts,
            };

            const result = await transform(args);

            const response = new resproto.TransformInvokeResponse();
            if (result === undefined) {
                response.setArgs(request.getArgs());
                response.setOptions(request.getOptions());
            } else {
                const margs = await serializeProperties("args", result.args);
                response.setArgs(gstruct.Struct.fromJavaScript(margs));

                // Copy the options over.
                if (result.opts !== undefined) {
                    opts = new resproto.TransformInvokeOptions();

                    if (result.opts.pluginDownloadURL !== undefined) {
                        opts.setPluginDownloadUrl(result.opts.pluginDownloadURL);
                    }
                    if (result.opts.provider !== undefined) {
                        const providerURN = await result.opts.provider.urn.promise();
                        const providerID = (await result.opts.provider.id.promise()) || unknownValue;
                        opts.setProvider(`${providerURN}::${providerID}`);
                    }
                    if (result.opts.version !== undefined) {
                        opts.setVersion(result.opts.version);
                    }

                    response.setOptions(opts);
                }
            }
            return response;
        };
        const tryCb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            try {
                return await cb(bytes);
            } catch (e) {
                throw new Error(`transform failed: ${e}`);
            }
        };
        const uuid = randomUUID();
        this._callbacks.set(uuid, tryCb);
        const req = new Callback();
        req.setToken(uuid);
        req.setTarget(await this._target);
        return req;
    }

    registerStackInvokeTransform(transform: InvokeTransform): void {
        this._pendingRegistrations++;

        this.registerStackInvokeTransformAsync(transform)
            .then(
                (req) => {
                    return new Promise((resolve, reject) => {
                        this._monitor.registerStackInvokeTransform(req, (err, _) => {
                            if (err !== null) {
                                // Remove this from the list of callbacks given we didn't manage to actually register it.
                                this._callbacks.delete(req.getToken());
                                reject();
                            } else {
                                resolve();
                            }
                        });
                    });
                },
                (err) => log.error(`failed to register stack transform: ${err}`),
            )
            .finally(() => {
                this._pendingRegistrations--;
                if (this._pendingRegistrations === 0) {
                    const queue = this._awaitQueue;
                    this._awaitQueue = [];
                    for (const waiter of queue) {
                        waiter();
                    }
                }
            });
    }

    async registerResourceHook(hook: ResourceHook): Promise<void> {
        const cb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            try {
                const request = resproto.ResourceHookRequest.deserializeBinary(bytes);
                const newInputs = request.getNewInputs();
                const oldInputs = request.getOldInputs();
                const newOutputs = request.getNewOutputs();
                const oldOutputs = request.getOldOutputs();
                await hook.callback({
                    urn: request.getUrn(),
                    id: request.getId(),
                    name: request.getName(),
                    type: request.getType(),
                    newInputs: newInputs ? deserializeProperties(newInputs, true /*keepUnknowns */) : undefined,
                    oldInputs: oldInputs ? deserializeProperties(oldInputs, true /*keepUnknowns */) : undefined,
                    newOutputs: newOutputs ? deserializeProperties(newOutputs, true /*keepUnknowns */) : undefined,
                    oldOutputs: oldOutputs ? deserializeProperties(oldOutputs, true /*keepUnknowns */) : undefined,
                });
            } catch (error) {
                const response = new resproto.ResourceHookResponse();
                response.setError(error.message);
                return response;
            }
            return new resproto.ResourceHookResponse();
        };

        const uuid = randomUUID();
        this._callbacks.set(uuid, cb);
        const callback = new Callback();
        callback.setToken(uuid);
        callback.setTarget(await this._target);

        const req = new resproto.RegisterResourceHookRequest();
        req.setCallback(callback);
        req.setName(hook.name);
        req.setOnDryRun(hook.opts?.onDryRun ?? false);

        const done = rpcKeepAlive();
        return debuggablePromise(
            new Promise((resolve, reject) => {
                this._monitor.registerResourceHook(req, (err, _) => {
                    if (err !== null) {
                        // Remove this from the list of callbacks given we didn't manage to actually register it.
                        this._callbacks.delete(uuid);
                        reject(err);
                    } else {
                        resolve();
                    }
                    done();
                });
            }),
            `resourceHook:${hook.name}`,
        );
    }
}
