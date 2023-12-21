// Copyright 2016-2023, Pulumi Corporation.
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
import { isUnknown, output } from "../output";
import * as aliasproto from "../proto/alias_pb";
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
    ResourceOptions,
    ResourceTransformation,
    ResourceTransformationArgs,
} from "../resource";
import { deserializeProperties, serializeProperties, unknownValue } from "./rpc";
import { getStackResource } from "./stack";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
/** @internal */
const maxRPCMessageSize: number = 1024 * 1024 * 400;

type CallbackFunction = (args: Uint8Array) => Promise<jspb.Message>;

export interface ICallbackServer {
    registerTransformation(callback: ResourceTransformation): Promise<callproto.Callback>;
    registerStackTransformation(callback: ResourceTransformation): void;
    shutdown(): void;
    // Wait for any pendind registerStackTransformation calls to complete.
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
            invoke: this.invoke,
        };
        this._server.addService(callrpc.CallbacksService, implementation);

        const self = this;
        this._target = new Promise<string>((resolve, reject) => {
            console.log("binding server");
            self._server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, port) => {
                console.log("bound server: ", port, err);
                if (err !== null) {
                    reject(err);
                    return;
                }
                self._server.start();

                // The server takes a while to _actually_ startup so we need to keep trying to send an invoke
                // to ourselves before we resolve the address to tell the engine about it.
                const target = `127.0.0.1:${port}`;

                const client = new callrpc.CallbacksClient(target, grpc.credentials.createInsecure());

                const connect = () => {
                    client.invoke(new CallbackInvokeRequest(), (error, _) => {
                        console.log(error);
                        if (error?.code === grpc.status.UNAVAILABLE) {
                            setTimeout(connect, 1000);
                            return;
                        }

                        resolve(target);
                    });
                };
                connect();
            });
        });
    }

    awaitStackRegistrations(): Promise<void> {
        console.log("awaitStackRegistrations: ", this._pendingRegistrations);
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
        console.log("Invoke called with request: ", call);
        const req = call.request;

        const cb = this._callbacks.get(req.getToken());
        if (cb === undefined) {
            console.log("callback not found: ", req.getToken());
            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.INVALID_ARGUMENT);
            err.withDetails("callback not found");
            callback(err.build());
            return;
        }

        try {
            console.log("calling callback");
            const response = await cb(req.getRequest_asU8());
            const resp = new CallbackInvokeResponse();
            console.log("callback response: ", resp);
            resp.setResponse(response.serializeBinary());
            callback(null, resp);
        } catch (e) {
            console.log("callback failed: ", e);

            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.UNKNOWN);
            if (e instanceof Error) {
                err.withDetails(e.message);
            } else {
                err.withDetails(JSON.stringify(e));
            }
            callback(err.build());
            return;
        }
    }

    async registerTransformation(transform: ResourceTransformation): Promise<callproto.Callback> {
        const cb = async (bytes: Uint8Array): Promise<jspb.Message> => {
            const request = resproto.TransformationRequest.deserializeBinary(bytes);

            const opts = request.getOptions() || new resproto.TransformationResourceOptions();

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
                    return {
                        name: spec.getName(),
                        type: spec.getType(),
                        project: spec.getProject(),
                        stack: spec.getStack(),
                        parent: spec.getParenturn() !== "" ? new DependencyResource(spec.getParenturn()) : undefined,
                    };
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

            const args: ResourceTransformationArgs = {
                // Remote transforms can't really synthasize a resource object here so we just pass the root stack object.
                resource: getStackResource()!,

                type: request.getType(),
                name: request.getName(),
                props: deserializeProperties(request.getProperties()),
                opts: ropts,
            };

            const result = transform(args);

            const response = new resproto.TransformationResponse();
            if (result === undefined) {
                response.setProperties(request.getProperties());
                response.setOptions(request.getOptions());
            } else {
                const mprops = await serializeProperties("props", result.props);
                response.setProperties(gstruct.Struct.fromJavaScript(mprops));

                // Copy the options over.
                if (result.opts !== undefined) {
                    if (result.opts.aliases !== undefined) {
                        const aliases = [];
                        for (const alias of result.opts.aliases) {
                            const resolved = await output(alias).promise(true);
                            if (isUnknown(resolved)) {
                                // Can't do anything with unknowns on options.
                                continue;
                            }

                            if (typeof resolved === "string") {
                                const a = new aliasproto.Alias();
                                a.setUrn(resolved);
                                aliases.push(a);
                            } else {
                                const spec = new aliasproto.Alias.Spec();
                                if (resolved.name !== undefined) {
                                    spec.setName(resolved.name);
                                }
                                if (resolved.type !== undefined) {
                                    spec.setType(resolved.type);
                                }
                                if (resolved.project !== undefined) {
                                    spec.setProject(resolved.project);
                                }
                                if (resolved.stack !== undefined) {
                                    spec.setStack(resolved.stack);
                                }
                                if (resolved.parent !== undefined) {
                                    if (Resource.isInstance(resolved.parent)) {
                                        spec.setParenturn(await resolved.parent.urn.promise());
                                    } else {
                                        spec.setParenturn(resolved.parent);
                                    }
                                }
                                const a = new aliasproto.Alias();
                                a.setSpec(spec);
                                aliases.push(a);
                            }
                        }
                        opts.setAliasesList(aliases);
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
                }
                response.setOptions(opts);
            }

            return response;
        };
        const uuid = randomUUID();
        this._callbacks.set(uuid, cb);
        const req = new Callback();
        req.setToken(uuid);
        req.setTarget(await this._target);
        console.log("registered transform: ", req);
        return req;
    }

    registerStackTransformation(transform: ResourceTransformation): void {
        this._pendingRegistrations++;
        console.log("registerStackTransformation: ", this._pendingRegistrations);

        this.registerTransformation(transform)
            .then(
                (req) => {
                    console.log("register stack transform: ", req);
                    this._monitor.registerStackTransformation(req, (err, _) => {
                        if (err !== null) {
                            log.error(`failed to register stack transformation: ${err.message}`);
                            return;
                        }
                        // Remove this from the list of callbacks given we didn't manage to actually register it.
                        this._callbacks.delete(req.getToken());
                    });
                },
                (err) => log.error(`failed to register stack transformation: ${err}`),
            )
            .finally(() => {
                this._pendingRegistrations--;
                console.log("registered stack transform: ", this._pendingRegistrations);
                if (this._pendingRegistrations === 0) {
                    const queue = this._awaitQueue;
                    this._awaitQueue = [];
                    for (const waiter of queue) {
                        waiter();
                    }
                }
            });
    }
}
