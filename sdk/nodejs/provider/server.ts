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

import * as grpc from "@grpc/grpc-js";

import { Provider } from "./provider";

import * as log from "../log";
import { Inputs, Output, output } from "../output";
import * as resource from "../resource";
import * as config from "../runtime/config";
import * as rpc from "../runtime/rpc";
import * as settings from "../runtime/settings";
import * as localState from "../runtime/state";
import { parseArgs } from "./internals";
import { InputPropertyError, InputPropertiesError, InputPropertyErrorDetails } from "../errors";

import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as anyproto from "google-protobuf/google/protobuf/any_pb";
import * as emptyproto from "google-protobuf/google/protobuf/empty_pb";
import * as structproto from "google-protobuf/google/protobuf/struct_pb";
import * as plugproto from "../proto/plugin_pb";
import * as provrpc from "../proto/provider_grpc_pb";
import * as provproto from "../proto/provider_pb";
import * as statusproto from "../proto/status_pb";
import * as errorproto from "../proto/errors_pb";

class Server implements grpc.UntypedServiceImplementation {
    engineAddr: string | undefined;
    readonly provider: Provider;
    readonly uncaughtErrors: Set<Error>;
    private readonly _callbacks = new Map<Symbol, grpc.sendUnaryData<any>>();

    constructor(engineAddr: string | undefined, provider: Provider, uncaughtErrors: Set<Error>) {
        this.engineAddr = engineAddr;
        this.provider = provider;
        this.uncaughtErrors = uncaughtErrors;

        // When we catch an uncaught error, we need to respond to the inflight call/construct gRPC requests
        // with the error to avoid a hang.
        const uncaughtHandler = (err: Error) => {
            if (!this.uncaughtErrors.has(err)) {
                this.uncaughtErrors.add(err);
            }
            // terminate the outstanding gRPC requests.
            this._callbacks.forEach((callback) => callback(err, undefined));
        };
        process.on("uncaughtException", uncaughtHandler);
        // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so
        // just suppress the TS strictness here.
        process.on("unhandledRejection", uncaughtHandler);
    }

    // Satisfy the grpc.UntypedServiceImplementation interface.
    [name: string]: any;

    // Misc. methods

    public cancel(call: any, callback: any): void {
        callback(undefined, new emptyproto.Empty());
    }

    public attach(call: any, callback: any): void {
        const req = call.request;
        const host = req.getAddress();
        this.engineAddr = host;
        callback(undefined, new emptyproto.Empty());
    }

    public getPluginInfo(call: any, callback: any): void {
        const resp: any = new plugproto.PluginInfo();
        resp.setVersion(this.provider.version);
        callback(undefined, resp);
    }

    public getSchema(call: any, callback: any): void {
        const req: any = call.request;
        if (req.getVersion() !== 0) {
            callback(new Error(`unsupported schema version ${req.getVersion()}`), undefined);
            return;
        }
        const resp: any = new provproto.GetSchemaResponse();
        if (this.provider.schema) {
            resp.setSchema(this.provider.schema);
            callback(undefined, resp);
        } else if (this.provider.getSchema) {
            this.provider
                .getSchema()
                .then((schema) => {
                    resp.setSchema(schema);
                    callback(undefined, resp);
                })
                .catch((err) => callback(err, undefined));
        } else {
            resp.setSchema("{}");
            callback(undefined, resp);
        }
    }

    // Config methods

    public checkConfig(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Not yet implemented: CheckConfig",
            },
            undefined,
        );
    }

    public diffConfig(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Not yet implemented: DiffConfig",
            },
            undefined,
        );
    }

    public configure(call: any, callback: any): void {
        const resp = new provproto.ConfigureResponse();
        resp.setAcceptsecrets(true);
        resp.setAcceptresources(true);
        resp.setAcceptoutputs(true);
        callback(undefined, resp);
    }

    public async parameterize(call: any, callback: any): Promise<void> {
        let res = null;
        if (call.request.hasArgs()) {
            if (!this.provider.parameterizeArgs) {
                callback(new Error("parameterizeArgs not implemented"), undefined);
                return;
            }

            res = await this.provider.parameterizeArgs(call.request.getArgs().getArgsList());
        } else {
            if (!this.provider.parameterizeValue) {
                callback(new Error("parameterizeValue not implemented"), undefined);
                return;
            }

            const b64 = call.request.getValue().getValue_asB64();
            res = await this.provider.parameterizeValue(
                call.request.getValue().getName(),
                call.request.getValue().getVersion(),
                Buffer.from(b64, "base64").toString("utf-8"),
            );
        }

        const resp = new provproto.ParameterizeResponse();
        resp.setName(res.name);
        resp.setVersion(res.version);
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
                resp.setInputs(
                    result.inputs === undefined ? undefined : structproto.Struct.fromJavaScript(result.inputs),
                );
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
                result = (await this.provider.update(req.getId(), req.getUrn(), olds, news)) || {};
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

    private buildInvalidPropertiesError(message: string, errors: Array<InputPropertyErrorDetails>): any {
        const metadata = new grpc.Metadata();
        if (errors) {
            const status = new statusproto.Status();
            // We don't care about the exact status code here, since they are pretty web centric, and don't
            // necessarily make sense in this context.  Pick one that's close enough.
            status.setCode(grpc.status.INVALID_ARGUMENT);
            status.setMessage(message);

            const errorDetails = new errorproto.InputPropertiesError();
            errors.forEach((detail) => {
                const propertyError = new errorproto.InputPropertiesError.PropertyError();
                propertyError.setPropertyPath(detail.propertyPath);
                propertyError.setReason(detail.reason);
                errorDetails.addErrors(propertyError);
            });

            const details = new anyproto.Any();
            details.pack(errorDetails.serializeBinary(), "pulumirpc.InputPropertiesError");

            status.addDetails(details);
            metadata.add("grpc-status-details-bin", Buffer.from(status.serializeBinary()));
        }
        const error = {
            code: grpc.status.INVALID_ARGUMENT,
            details: message,
            metadata: metadata,
        };
        return error;
    }

    public async construct(call: any, callback: any): Promise<void> {
        // Setup a new async state store for this run
        const store = new localState.LocalStore();
        return localState.asyncLocalStorage.run(store, async () => {
            const callbackId = Symbol("id");
            this._callbacks.set(callbackId, callback);
            try {
                const req: any = call.request;
                const type = req.getType();
                const name = req.getName();

                if (!this.provider.construct) {
                    callback(new Error(`unknown resource type ${type}`), undefined);
                    return;
                }

                await configureRuntime(req, this.engineAddr);

                const inputs = await deserializeInputs(req.getInputs(), req.getInputdependenciesMap());

                // Rebuild the resource options.
                const dependsOn: resource.Resource[] = [];
                for (const urn of req.getDependenciesList()) {
                    dependsOn.push(new resource.DependencyResource(urn));
                }
                const providers: Record<string, resource.ProviderResource> = {};
                const rpcProviders = req.getProvidersMap();
                if (rpcProviders) {
                    for (const [pkg, ref] of rpcProviders.entries()) {
                        providers[pkg] = createProviderResource(ref);
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

                const [state, stateDependencies] = await rpc.serializeResourceProperties(
                    `construct(${type}, ${name})`,
                    result.state,
                );
                const stateDependenciesMap = resp.getStatedependenciesMap();
                for (const [key, resources] of stateDependencies) {
                    const deps = new provproto.ConstructResponse.PropertyDependencies();
                    deps.setUrnsList(await Promise.all(Array.from(resources).map((r) => r.urn.promise())));
                    stateDependenciesMap.set(key, deps);
                }
                resp.setState(structproto.Struct.fromJavaScript(state));

                // Wait for RPC operations to complete.
                await settings.waitForRPCs();

                callback(undefined, resp);
            } catch (e) {
                if (InputPropertiesError.isInstance(e)) {
                    const error = this.buildInvalidPropertiesError(e.message, e.errors);
                    callback(error, undefined);
                    return;
                } else if (InputPropertyError.isInstance(e)) {
                    const error = this.buildInvalidPropertiesError("", [
                        { propertyPath: e.propertyPath, reason: e.reason },
                    ]);
                    callback(error, undefined);
                    return;
                }
                callback(e, undefined);
            } finally {
                // remove the gRPC callback context from the map of in-flight callbacks
                this._callbacks.delete(callbackId);
            }
        });
    }

    public async call(call: any, callback: any): Promise<void> {
        // Setup a new async state store for this run
        const store = new localState.LocalStore();
        return localState.asyncLocalStorage.run(store, async () => {
            const callbackId = Symbol("id");
            this._callbacks.set(callbackId, callback);
            try {
                const req: any = call.request;
                if (!this.provider.call) {
                    callback(new Error(`unknown function ${req.getTok()}`), undefined);
                    return;
                }

                await configureRuntime(req, this.engineAddr);

                const args = await deserializeInputs(req.getArgs(), req.getArgdependenciesMap());

                const result = await this.provider.call(req.getTok(), args);

                const resp = new provproto.CallResponse();

                if (result.outputs) {
                    const [ret, retDependencies] = await rpc.serializeResourceProperties(
                        `call(${req.getTok()})`,
                        result.outputs,
                    );
                    const returnDependenciesMap = resp.getReturndependenciesMap();
                    for (const [key, resources] of retDependencies) {
                        const deps = new provproto.CallResponse.ReturnDependencies();
                        deps.setUrnsList(await Promise.all(Array.from(resources).map((r) => r.urn.promise())));
                        returnDependenciesMap.set(key, deps);
                    }
                    resp.setReturn(structproto.Struct.fromJavaScript(ret));
                }

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

                // Wait for RPC operations to complete.
                await settings.waitForRPCs();

                callback(undefined, resp);
            } catch (e) {
                if (InputPropertiesError.isInstance(e)) {
                    const error = this.buildInvalidPropertiesError(e.message, e.errors);
                    callback(error, undefined);
                    return;
                } else if (InputPropertyError.isInstance(e)) {
                    const error = this.buildInvalidPropertiesError("", [
                        { propertyPath: e.propertyPath, reason: e.reason },
                    ]);
                    callback(error, undefined);
                    return;
                }
                callback(e, undefined);
            } finally {
                // remove the gRPC callback context from the map of in-flight callbacks
                this._callbacks.delete(callbackId);
            }
        });
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
            resp.setReturn(structproto.Struct.fromJavaScript(result.outputs));

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
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Not yet implemented: StreamInvoke",
            },
            undefined,
        );
    }
}

async function configureRuntime(req: any, engineAddr: string | undefined) {
    // NOTE: these are globals! We should ensure that all settings are identical between calls, and eventually
    // refactor so we can avoid the global state.
    if (engineAddr === undefined) {
        throw new Error("fatal: Missing <engine> address");
    }

    settings.resetOptions(
        req.getProject(),
        req.getStack(),
        req.getParallel(),
        engineAddr,
        req.getMonitorendpoint(),
        req.getDryrun(),
        req.getOrganization(),
    );

    // resetOptions doesn't reset the saved features
    await settings.awaitFeatureSupport();

    const pulumiConfig: { [key: string]: string } = {};
    const rpcConfig = req.getConfigMap();
    if (rpcConfig) {
        for (const [k, v] of rpcConfig.entries()) {
            pulumiConfig[k] = v;
        }
    }
    config.setAllConfig(pulumiConfig, req.getConfigsecretkeysList());
}

/**
 * Deserializes the inputs struct and applies appropriate dependencies.
 *
 * @internal
 */
export async function deserializeInputs(inputsStruct: gstruct.Struct, inputDependencies: any): Promise<Inputs> {
    const result: Inputs = {};

    const deserializedInputs = rpc.deserializeProperties(inputsStruct);
    for (const k of Object.keys(deserializedInputs)) {
        const input = deserializedInputs[k];
        const isSecret = rpc.isRpcSecret(input);
        const depsUrns: resource.URN[] = inputDependencies.get(k)?.getUrnsList() ?? [];

        if (
            !isSecret &&
            (depsUrns.length === 0 || containsOutputs(input) || (await isResourceReference(input, depsUrns)))
        ) {
            // If the input isn't a secret and either doesn't have any dependencies, already contains Outputs (from
            // deserialized output values), or is a resource reference, then we can return it directly without
            // wrapping it as an output.
            result[k] = input;
        } else {
            // Otherwise, wrap it in an output so we can handle secrets and/or track dependencies.
            // Note: If the value is or contains an unknown value, the Output will mark its value as
            // unknown automatically, so we just pass true for isKnown here.
            const deps = depsUrns.map((depUrn) => new resource.DependencyResource(depUrn));
            result[k] = new Output(
                deps,
                Promise.resolve(rpc.unwrapRpcSecret(input)),
                Promise.resolve(true),
                Promise.resolve(isSecret),
                Promise.resolve([]),
            );
        }
    }

    return result;
}

/**
 * Returns true if the input is a resource reference.
 */
async function isResourceReference(input: any, deps: string[]): Promise<boolean> {
    return resource.Resource.isInstance(input) && deps.length === 1 && deps[0] === (await input.urn.promise());
}

/**
 * Returns true if the deserialized input contains outputs (deeply), excluding
 * properties of resources.
 *
 * @internal
 */
export function containsOutputs(input: any): boolean {
    if (Array.isArray(input)) {
        for (const e of input) {
            if (containsOutputs(e)) {
                return true;
            }
        }
    } else if (typeof input === "object") {
        if (Output.isInstance(input)) {
            return true;
        } else if (resource.Resource.isInstance(input)) {
            // Do not drill into instances of Resource because they will have properties that are
            // instances of Output (e.g. urn, id, etc.) and we're only looking for instances of
            // Output that aren't associated with a Resource.
            return false;
        }

        for (const k of Object.keys(input)) {
            if (containsOutputs(input[k])) {
                return true;
            }
        }
    }
    return false;
}

/**
 * Creates a gRPC response representing an error from a dynamic provider's
 * resource. This is typically either a creation error, in which the API server
 * has (virtually) rejected the resource, or an initialization error, where the
 * API server has accepted the resource, but it failed to initialize (e.g., the
 * app code is continually crashing and the resource has failed to become
 * alive).
 */
function grpcResponseFromError(e: { id: string; properties: any; message: string; reasons?: string[] }) {
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
            log.error(err.stack || err.message || "" + err);
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

    const parsedArgs = parseArgs(args);

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const server = new grpc.Server({
        "grpc.max_receive_message_length": settings.maxRPCMessageSize,
    });

    // The program receives a single optional argument: the address of the RPC endpoint for the engine.  It
    // optionally also takes a second argument, a reference back to the engine, but this may be missing.

    const engineAddr = parsedArgs?.engineAddress;
    server.addService(provrpc.ResourceProviderService, new Server(engineAddr, provider, uncaughtErrors));
    const port: number = await new Promise<number>((resolve, reject) => {
        server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
            if (err) {
                reject(err);
            } else {
                resolve(p);
            }
        });
    });

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    // We explicitly convert the number to a string so that Node doesn't colorize the output.
    console.log(port.toString());
}

/**
 * Rehydrate the provider reference into a registered ProviderResource,
 * otherwise return an instance of DependencyProviderResource.
 */
function createProviderResource(ref: string): resource.ProviderResource {
    const [urn] = resource.parseResourceReference(ref);
    const urnParts = urn.split("::");
    const qualifiedType = urnParts[2];
    const urnName = urnParts[3];

    const type = qualifiedType.split("$").pop()!;
    const typeParts = type.split(":");
    const typName = typeParts.length > 2 ? typeParts[2] : "";

    const resourcePackage = rpc.getResourcePackage(typName, /*version:*/ "");
    if (resourcePackage) {
        return resourcePackage.constructProvider(urnName, type, urn);
    }
    return new resource.DependencyProviderResource(ref);
}
