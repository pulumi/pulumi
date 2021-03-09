// Copyright 2016-2020, Pulumi Corporation.
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

import * as minimist from "minimist";
import * as path from "path";

import * as grpc from "@grpc/grpc-js";

import { Provider } from "./provider";

import * as log from "../log";
import { Inputs, Output, output } from "../output";
import * as resource from "../resource";
import * as runtime from "../runtime";
import { version } from "../version";

const requireFromString = require("require-from-string");
const anyproto = require("google-protobuf/google/protobuf/any_pb.js");
const emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");
const provproto = require("../proto/provider_pb.js");
const provrpc = require("../proto/provider_grpc_pb.js");
const plugproto = require("../proto/plugin_pb.js");
const statusproto = require("../proto/status_pb.js");

class Server implements grpc.UntypedServiceImplementation {
    readonly engineAddr: string;
    readonly provider: Provider;

    /** Queue of construct calls. */
    constructQueue = Promise.resolve();

    constructor(engineAddr: string, provider: Provider) {
        this.engineAddr = engineAddr;
        this.provider = provider;
    }

    // Satisfy the grpc.UntypedServiceImplementation interface.
    [name: string]: any;

    // Misc. methods

    public cancel(call: any, callback: any): void {
        callback(undefined, new emptyproto.Empty());
    }

    public getPluginInfo(call: any, callback: any): void {
        const resp: any = new plugproto.PluginInfo();
        resp.setVersion(this.provider.version);
        callback(undefined, resp);
    }

    public getSchema(call: any, callback: any): void {
        callback({
            code: grpc.status.UNIMPLEMENTED,
            details: "Not yet implemented: GetSchema",
        }, undefined);
    }

    // Config methods

    public checkConfig(call: any, callback: any): void {
        callback({
            code: grpc.status.UNIMPLEMENTED,
            details: "Not yet implemented: CheckConfig",
        }, undefined);
    }

    public diffConfig(call: any, callback: any): void {
        callback({
            code: grpc.status.UNIMPLEMENTED,
            details: "Not yet implemented: DiffConfig",
        }, undefined);
    }

    public configure(call: any, callback: any): void {
        const resp = new provproto.ConfigureResponse();
        resp.setAcceptsecrets(true);
        resp.setAcceptresources(true);
        callback(undefined, resp);
    }

    // CRUD resource methods

    public async check(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.CheckResponse();

            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();

            let inputs: any = news;
            let failures: any[] = [];
            if (this.provider.check) {
                const result = await this.provider.check(req.getUrn(), olds, news);
                if (result.inputs) {
                    inputs = result.inputs;
                }
                if (result.failures) {
                    failures = result.failures;
                }
            } else {
                // If no check method was provided, propagate the new inputs as-is.
                inputs = news;
            }

            resp.setInputs(structproto.Struct.fromJavaScript(inputs));

            if (failures.length !== 0) {
                const failureList = [];
                for (const f of failures) {
                    const failure = new provproto.CheckFailure();
                    failure.setProperty(f.property);
                    failure.setReason(f.reason);
                    failureList.push(failure);
                }
                resp.setFailuresList(failureList);
            }

            callback(undefined, resp);
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async diff(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.DiffResponse();

            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();
            if (this.provider.diff) {
                const result: any = await this.provider.diff(req.getId(), req.getUrn(), olds, news);

                if (result.changes === true) {
                    resp.setChanges(provproto.DiffResponse.DiffChanges.DIFF_SOME);
                } else if (result.changes === false) {
                    resp.setChanges(provproto.DiffResponse.DiffChanges.DIFF_NONE);
                } else {
                    resp.setChanges(provproto.DiffResponse.DiffChanges.DIFF_UNKNOWN);
                }

                if (result.replaces && result.replaces.length !== 0) {
                    resp.setReplacesList(result.replaces);
                }
                if (result.deleteBeforeReplace) {
                    resp.setDeletebeforereplace(result.deleteBeforeReplace);
                }
            }

            callback(undefined, resp);
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async create(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            if (!this.provider.create) {
                callback(new Error(`unknown resource type ${req.getUrn()}`), undefined);
                return;
            }

            const resp = new provproto.CreateResponse();
            const props = req.getProperties().toJavaScript();
            const result = await this.provider.create(req.getUrn(), props);
            resp.setId(result.id);
            resp.setProperties(structproto.Struct.fromJavaScript(result.outs));

            callback(undefined, resp);
        } catch (e) {
            const response = grpcResponseFromError(e);
            return callback(/*err:*/ response, /*value:*/ null, /*metadata:*/ response.metadata);
        }
    }

    public async read(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.ReadResponse();

            const id = req.getId();
            const props = req.getProperties().toJavaScript();
            if (this.provider.read) {
                const result: any = await this.provider.read(id, req.getUrn(), props);
                resp.setId(result.id);
                resp.setProperties(structproto.Struct.fromJavaScript(result.props));
            } else {
                // In the event of a missing read, simply return back the input state.
                resp.setId(id);
                resp.setProperties(req.getProperties());
            }

            callback(undefined, resp);
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async update(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.UpdateResponse();

            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();

            let result: any = {};
            if (this.provider.update) {
                result = await this.provider.update(req.getId(), req.getUrn(), olds, news) || {};
            }

            resp.setProperties(structproto.Struct.fromJavaScript(result.outs));

            callback(undefined, resp);
        } catch (e) {
            const response = grpcResponseFromError(e);
            return callback(/*err:*/ response, /*value:*/ null, /*metadata:*/ response.metadata);
        }
    }

    public async delete(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const props: any = req.getProperties().toJavaScript();
            if (this.provider.delete) {
                await this.provider.delete(req.getId(), req.getUrn(), props);
            }
            callback(undefined, new emptyproto.Empty());
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async construct(call: any, callback: any): Promise<void> {
        // Serialize invocations of `construct` so that each call runs one after another, avoiding concurrent runs.
        // We do this because `construct` has to modify global state to reset the SDK's runtime options.
        // This is a short-term workaround to provide correctness, but likely isn't sustainable long-term due to the
        // limits it places on parallelism. We will likely want to investigate if it's possible to run each invocation
        // in its own context, possibly using Node's `createContext` API:
        // https://nodejs.org/api/vm.html#vm_vm_createcontext_contextobject_options
        const res = this.constructQueue.then(() => this.constructImpl(call, callback));
        // tslint:disable:no-empty
        this.constructQueue = res.catch(() => {});
        return res;
    }

    async constructImpl(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const type = req.getType();
            const name = req.getName();

            if (!this.provider.construct) {
                callback(new Error(`unknown resource type ${type}`), undefined);
                return;
            }

            // Configure the runtime.
            //
            // NOTE: these are globals! We should ensure that all settings are identical between calls, and eventually
            // refactor so we can avoid the global state.
            runtime.resetOptions(req.getProject(), req.getStack(), req.getParallel(), this.engineAddr,
                                        req.getMonitorendpoint(), req.getDryrun());

            const pulumiConfig: {[key: string]: string} = {};
            const rpcConfig = req.getConfigMap();
            if (rpcConfig) {
                for (const [k, v] of rpcConfig.entries()) {
                    pulumiConfig[k] = v;
                }
            }
            runtime.setAllConfig(pulumiConfig);

            // Deserialize the inputs and apply appropriate dependencies.
            const inputs: Inputs = {};
            const inputDependencies = req.getInputdependenciesMap();
            const deserializedInputs = runtime.deserializeProperties(req.getInputs());
            for (const k of Object.keys(deserializedInputs)) {
                const inputDeps = inputDependencies.get(k);
                const deps = (inputDeps ? <resource.URN[]>inputDeps.getUrnsList() : [])
                    .map(depUrn => new resource.DependencyResource(depUrn));
                const input = deserializedInputs[k];
                const isSecret = runtime.isRpcSecret(input);
                if (!isSecret && deps.length === 0) {
                    // If it's a prompt value, return it directly without wrapping it as an output.
                    inputs[k] = input;
                } else {
                    // Otherwise, wrap it in an output so we can handle secrets and/or track dependencies.
                    inputs[k] = new Output(deps, Promise.resolve(runtime.unwrapRpcSecret(input)), Promise.resolve(true),
                        Promise.resolve(isSecret), Promise.resolve([]));
                }
            }

            // Rebuild the resource options.
            const dependsOn: resource.Resource[] = [];
            for (const urn of req.getDependenciesList()) {
                dependsOn.push(new resource.DependencyResource(urn));
            }
            const providers: Record<string, resource.ProviderResource> = {};
            const rpcProviders = req.getProvidersMap();
            if (rpcProviders) {
                for (const [pkg, ref] of rpcProviders.entries()) {
                    providers[pkg] = new resource.DependencyProviderResource(ref);
                }
            }
            const opts: resource.ComponentResourceOptions = {
                aliases: req.getAliasesList(),
                dependsOn: dependsOn,
                protect: req.getProtect(),
                providers: providers,
                parent: req.getParent() ? new resource.DependencyResource(req.getParent()) : undefined,
            };

            const result = await this.provider.construct(name, type, inputs, opts);

            const resp = new provproto.ConstructResponse();

            resp.setUrn(await output(result.urn).promise());

            const [state, stateDependencies] = await runtime.serializeResourceProperties(`construct(${type}, ${name})`, result.state);
            const stateDependenciesMap = resp.getStatedependenciesMap();
            for (const [key, resources] of stateDependencies) {
                const deps = new provproto.ConstructResponse.PropertyDependencies();
                deps.setUrnsList(await Promise.all(Array.from(resources).map(r => r.urn.promise())));
                stateDependenciesMap.set(key, deps);
            }
            resp.setState(structproto.Struct.fromJavaScript(state));

            // Wait for RPC operations to complete and disconnect.
            await runtime.disconnect();

            callback(undefined, resp);
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async invoke(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            if (!this.provider.invoke) {
                callback(new Error(`unknown function ${req.getTok()}`), undefined);
                return;
            }


            const args: any = req.getArgs().toJavaScript();
            const result = await this.provider.invoke(req.getTok(), args);

            const resp = new provproto.InvokeResponse();
            resp.setProperties(structproto.Struct.fromJavaScript(result.outputs));

            if ((result.failures || []).length !== 0) {
                const failureList = [];
                for (const f of result.failures!) {
                    const failure = new provproto.CheckFailure();
                    failure.setProperty(f.property);
                    failure.setReason(f.reason);
                    failureList.push(failure);
                }
                resp.setFailuresList(failureList);
            }

            callback(undefined, resp);
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    public async streamInvoke(call: any, callback: any): Promise<void> {
        callback({
            code: grpc.status.UNIMPLEMENTED,
            details: "Not yet implemented: StreamInvoke",
        }, undefined);
    }
}

// grpcResponseFromError creates a gRPC response representing an error from a dynamic provider's
// resource. This is typically either a creation error, in which the API server has (virtually)
// rejected the resource, or an initialization error, where the API server has accepted the
// resource, but it failed to initialize (e.g., the app code is continually crashing and the
// resource has failed to become alive).
function grpcResponseFromError(e: {id: string, properties: any, message: string, reasons?: string[]}) {
    // Create response object.
    const resp = new statusproto.Status();
    resp.setCode(grpc.status.UNKNOWN);
    resp.setMessage(e.message);

    const metadata = new grpc.Metadata();
    if (e.id) {
        // Object created successfully, but failed to initialize. Pack initialization failure into
        // details.
        const detail = new provproto.ErrorResourceInitFailed();
        detail.setId(e.id);
        detail.setProperties(structproto.Struct.fromJavaScript(e.properties || {}));
        detail.setReasonsList(e.reasons || []);

        const details = new anyproto.Any();
        details.pack(detail.serializeBinary(), "pulumirpc.ErrorResourceInitFailed");

        // Add details to metadata.
        resp.addDetails(details);
        // NOTE: `grpc-status-details-bin` is a magic field that allows us to send structured
        // protobuf data as an error back through gRPC. This notion of details is a first-class in
        // the Go gRPC implementation, and the nodejs implementation has not quite caught up to it,
        // which is why it's cumbersome here.
        metadata.add("grpc-status-details-bin", Buffer.from(resp.serializeBinary()));
    }

    return {
        code: grpc.status.UNKNOWN,
        message: e.message,
        metadata: metadata,
    };
}

export async function main(provider: Provider, args: string[]) {
    // We track all uncaught errors here.  If we have any, we will make sure we always have a non-0 exit
    // code.
    const uncaughtErrors = new Set<Error>();
    const uncaughtHandler = (err: Error) => {
        if (!uncaughtErrors.has(err)) {
            uncaughtErrors.add(err);
            // Use `pulumi.log.error` here to tell the engine there was a fatal error, which should
            // stop processing subsequent resource operations.
            log.error(err.stack || err.message || ("" + err));
        }
    };

    process.on("uncaughtException", uncaughtHandler);
    // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so just
    // suppress the TS strictness here.
    process.on("unhandledRejection", uncaughtHandler);
    process.on("exit", (code: number) => {
        // If there were any uncaught errors at all, we always want to exit with an error code.
        if (code === 0 && uncaughtErrors.size > 0) {
            process.exitCode = 1;
        }
    });

    // The program requires a single argument: the address of the RPC endpoint for the engine.  It
    // optionally also takes a second argument, a reference back to the engine, but this may be missing.
    if (args.length === 0) {
        console.error("fatal: Missing <engine> address");
        process.exit(-1);
        return;
    }
    const engineAddr: string = args[0];

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const server = new grpc.Server({
        "grpc.max_receive_message_length": runtime.maxRPCMessageSize,
    });
    server.addService(provrpc.ResourceProviderService, new Server(engineAddr, provider));
    const port: number = await new Promise<number>((resolve, reject) => {
        server.bindAsync(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
            if (err) {
                reject(err);
            } else {
                resolve(p);
            }
        });
    });
    server.start();

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}
