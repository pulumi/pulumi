// Copyright 2016-2022, Pulumi Corporation.
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

// Enable source map support so we get good stack traces.
import "source-map-support/register";

import * as grpc from "@grpc/grpc-js";

import { getProject } from "../../metadata";
import * as dynamic from "../../dynamic";
import * as rpc from "../../runtime/rpc";
import { version } from "../../version";
import * as dynConfig from "./config";

import * as anyproto from "google-protobuf/google/protobuf/any_pb";
import * as emptyproto from "google-protobuf/google/protobuf/empty_pb";
import * as structproto from "google-protobuf/google/protobuf/struct_pb";
import * as plugproto from "../../proto/plugin_pb";
import * as provrpc from "../../proto/provider_grpc_pb";
import * as provproto from "../../proto/provider_pb";
import * as statusproto from "../../proto/status_pb";

const requireFromString = require("require-from-string");

const providerKey: string = "__provider";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
const maxRPCMessageSize: number = 1024 * 1024 * 400;

// We track all uncaught errors here.  If we have any, we will make sure we always have a non-0 exit
// code.
const uncaughtErrors = new Set<Error>();
const uncaughtHandler = (err: Error) => {
    if (!uncaughtErrors.has(err)) {
        uncaughtErrors.add(err);
        console.error(err.stack || err.message || "" + err);
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

const providerCache: Record<string, dynamic.ResourceProvider> = {};

/**
 * getProvider deserializes the provider from the string found in
 * `props[providerKey]` and calls `provider.configure` with the config. The
 * deserialized and configured provider is stored in `providerCache`. This
 * guarantees that the provider is only deserialized and configured once per
 * process.
 */
async function getProvider(props: any, rawConfig: Record<string, any>): Promise<dynamic.ResourceProvider> {
    const providerString = props[providerKey];
    let provider: any = providerCache[providerString];
    if (!provider) {
        provider = requireFromString(providerString).handler();
        providerCache[providerString] = provider;
        if (provider.configure) {
            const config = new dynConfig.Config(rawConfig, getProject());
            const req: dynamic.ConfigureRequest = { config };
            await provider.configure(req);
        }
    }

    // TODO[pulumi/pulumi#414]: investigate replacing requireFromString with eval
    return provider;
}

// Each of the *RPC functions below implements a single method of the resource provider gRPC interface. The CRUD
// functions--checkRPC, diffRPC, createRPC, updateRPC, and deleteRPC--all operate in a similar fashion:
//     1. Deserialize the dyanmic provider for the resource on which the function is operating
//     2. Call the dynamic provider's corresponding {check,diff,create,update,delete} method
//     3. Convert and return the results
// In all cases, the dynamic provider is available in its serialized form as a property of the resource;
// getProvider` is responsible for handling its deserialization. In the case of diffRPC, if the provider itself
// has changed, `diff` reports that the resource requires replacement and does not delegate to the dynamic provider.
// This allows the creation of the replacement resource to use the new provider while the deletion of the old
// resource uses the provider with which it was created.

class ResourceProviderService implements provrpc.IResourceProviderServer {
    [method: string]: grpc.UntypedHandleCall;

    // Ideally we'd type the config as `Record<string, any>`, but since we
    // implement `IResourceProviderServer`, we require the `[method: string`]
    // index signature to satisfy the interface. This is a bit unfortunate and
    // means we can't have a strongly typed `config` property. We'll just use
    // `any` here.
    private config: any;

    cancel(call: any, callback: any): void {
        callback(undefined, new emptyproto.Empty());
    }

    async configure(
        call: grpc.ServerUnaryCall<provproto.ConfigureRequest, provproto.ConfigureResponse>,
        callback: any,
    ): Promise<void> {
        const protoArgs = call.request.getArgs();
        if (!protoArgs) {
            throw new Error("ConfigureRequest missing args");
        }
        const args = protoArgs.toJavaScript();
        const config: Record<string, any> = {};
        for (const [k, v] of Object.entries(args)) {
            if (k === providerKey) {
                continue;
            }
            config[k] = rpc.unwrapRpcSecret(v);
        }
        this.config = config;
        const resp = new provproto.ConfigureResponse();
        resp.setAcceptsecrets(false);
        callback(undefined, resp);
    }

    async invoke(call: any, callback: any): Promise<void> {
        const req: any = call.request;

        // TODO[pulumi/pulumi#406]: implement this.
        callback(new Error(`unknown function ${req.getTok()}`), undefined);
    }

    async streamInvoke(
        call: grpc.ServerWritableStream<provproto.InvokeRequest, provproto.InvokeResponse>,
    ): Promise<void> {
        const req: any = call.request;

        // TODO[pulumi/pulumi#406]: implement this.
        call.emit("error", {
            code: grpc.status.UNIMPLEMENTED,
            details: `unknown function ${req.getTok()}`,
        });
    }

    async check(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.CheckResponse();

            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();
            const provider = await getProvider(news[providerKey] === rpc.unknownValue ? olds : news, this.config);

            let inputs: any = news;
            let failures: any[] = [];
            if (provider.check) {
                const result = await provider.check(olds, news);
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

            inputs[providerKey] = news[providerKey];
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

    checkConfig(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "CheckConfig is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    async diff(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.DiffResponse();

            // Note that we do not take any special action if the provider has changed. This allows a user to iterate on a
            // dynamic provider's implementation. This does require some care on the part of the user: each iteration of a
            // dynamic provider's implementation must be able to handle all state produced by prior iterations.
            //
            // Prior versions of the dynamic provider required that a dynamic resource be replaced any time its provider
            // implementation changed. This made iteration painful, especially if the dynamic resource was managing a
            // physical resource--in this case, the physical resource would be unnecessarily deleted and recreated each
            // time the provider was updated.
            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();
            const provider = await getProvider(news[providerKey] === rpc.unknownValue ? olds : news, this.config);
            if (provider.diff) {
                const result: any = await provider.diff(req.getId(), olds, news);

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

    diffConfig(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "DiffConfig is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    async create(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.CreateResponse();

            const props = req.getProperties().toJavaScript();
            const provider = await getProvider(props, this.config);
            const result = await provider.create(props);
            const resultProps = resultIncludingProvider(result.outs, props);
            resp.setId(result.id);
            resp.setProperties(structproto.Struct.fromJavaScript(resultProps));

            callback(undefined, resp);
        } catch (e) {
            const response = grpcResponseFromError(e);
            return callback(/*err:*/ response, /*value:*/ null, /*metadata:*/ response.metadata);
        }
    }

    async read(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.ReadResponse();

            const id = req.getId();
            const props = req.getProperties().toJavaScript();
            const provider = await getProvider(props, this.config);
            if (provider.read) {
                // If there's a read function, consult the provider. Ensure to propagate the special __provider
                // value too, so that the provider's CRUD operations continue to function after a refresh.
                const result: any = await provider.read(id, props);
                resp.setId(result.id);
                const resultProps = resultIncludingProvider(result.props, props);
                resp.setProperties(structproto.Struct.fromJavaScript(resultProps));
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

    async update(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const resp = new provproto.UpdateResponse();

            const olds = req.getOlds().toJavaScript();
            const news = req.getNews().toJavaScript();

            let result: any = {};
            const provider = await getProvider(news, this.config);
            if (provider.update) {
                result = (await provider.update(req.getId(), olds, news)) || {};
            }

            const resultProps = resultIncludingProvider(result.outs, news);
            resp.setProperties(structproto.Struct.fromJavaScript(resultProps));

            callback(undefined, resp);
        } catch (e) {
            const response = grpcResponseFromError(e);
            return callback(/*err:*/ response, /*value:*/ null, /*metadata:*/ response.metadata);
        }
    }

    async delete(call: any, callback: any): Promise<void> {
        try {
            const req: any = call.request;
            const props: any = req.getProperties().toJavaScript();
            const provider: any = await getProvider(props, this.config);
            if (provider.delete) {
                await provider.delete(req.getId(), props);
            }
            callback(undefined, new emptyproto.Empty());
        } catch (e) {
            console.error(`${e}: ${e.stack}`);
            callback(e, undefined);
        }
    }

    async getPluginInfo(call: any, callback: any): Promise<void> {
        const resp: any = new plugproto.PluginInfo();
        resp.setVersion(version);
        callback(undefined, resp);
    }

    handshake(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Handshake is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    getSchema(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "GetSchema is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    construct(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Construct is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    parameterize(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Parameterize is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    call(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Call is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    attach(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Attach is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    getMapping(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "GetMapping is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    getMappings(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "GetMappings is not implemented by the dynamic provider",
            },
            undefined,
        );
    }

    migrate(call: any, callback: any): void {
        callback(
            {
                code: grpc.status.UNIMPLEMENTED,
                details: "Migrate is not implemented by the dynamic provider",
            },
            undefined,
        );
    }
}

function resultIncludingProvider(result: any, props: any): any {
    return Object.assign(result || {}, {
        [providerKey]: props[providerKey],
    });
}

/**
 * grpcResponseFromError creates a gRPC response representing an error from a dynamic provider's
 * resource. This is typically either a creation error, in which the API server has (virtually)
 * rejected the resource, or an initialization error, where the API server has accepted the
 * resource, but it failed to initialize (e.g., the app code is continually crashing and the
 * resource has failed to become alive).
 */
function grpcResponseFromError(e: {
    id: string;
    properties: any;
    message: string;
    reasons?: string[];
}) {
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

/** @internal */
export async function main(args: string[]) {
    // The program requires a single argument: the address of the RPC endpoint for the engine.  It
    // optionally also takes a second argument, a reference back to the engine, but this may be missing.
    if (args.length === 0) {
        console.error("fatal: Missing <engine> address");
        process.exit(-1);
    }

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const server = new grpc.Server({
        "grpc.max_receive_message_length": maxRPCMessageSize,
    });
    const resourceProvider = new ResourceProviderService();
    server.addService(provrpc.ResourceProviderService, resourceProvider);
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

main(process.argv.slice(2));
