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

import * as minimist from "minimist";
import * as path from "path";

import * as grpc from "@grpc/grpc-js";

import * as dynamic from "../../dynamic";
import * as resource from "../../resource";
import * as runtime from "../../runtime";
import { version } from "../../version";

const requireFromString = require("require-from-string");
const anyproto = require("google-protobuf/google/protobuf/any_pb.js");
const emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");
const provproto = require("../../proto/provider_pb.js");
const provrpc = require("../../proto/provider_grpc_pb.js");
const plugproto = require("../../proto/plugin_pb.js");
const statusproto = require("../../proto/status_pb.js");

const providerKey: string = "__provider";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
const maxRPCMessageSize: number = 1024 * 1024 * 400;

// We track all uncaught errors here.  If we have any, we will make sure we always have a non-0 exit
// code.
const uncaughtErrors = new Set<Error>();
const uncaughtHandler = (err: Error) => {
    if (!uncaughtErrors.has(err)) {
        uncaughtErrors.add(err);
        console.error(err.stack || err.message || ("" + err));
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

const providerCache: { [key: string]: dynamic.ResourceProvider } = {};

function getProvider(props: any): dynamic.ResourceProvider {
    const providerString = props[providerKey];
    let provider: any = providerCache[providerString];
    if (!provider) {
        provider = requireFromString(providerString).handler();
        providerCache[providerString] = provider;
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

function cancelRPC(call: any, callback: any): void {
    callback(undefined, new emptyproto.Empty());
}

function configureRPC(call: any, callback: any): void {
    const resp = new provproto.ConfigureResponse();
    resp.setAcceptsecrets(false);
    callback(undefined, resp);
}

async function invokeRPC(call: any, callback: any): Promise<void> {
    const req: any = call.request;

    // TODO[pulumi/pulumi#406]: implement this.
    callback(new Error(`unknown function ${req.getTok()}`), undefined);
}

async function streamInvokeRPC(call: any, callback: any): Promise<void> {
    const req: any = call.request;

    // TODO[pulumi/pulumi#406]: implement this.
    callback(new Error(`unknown function ${req.getTok()}`), undefined);
}

async function checkRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.CheckResponse();

        const olds = req.getOlds().toJavaScript();
        const news = req.getNews().toJavaScript();
        const provider = getProvider(news[providerKey] === runtime.unknownValue ? olds : news);

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

function checkConfigRPC(call: any, callback: any): void {
    callback({
        code: grpc.status.UNIMPLEMENTED,
        details: "CheckConfig is not implemented by the dynamic provider",
    }, undefined);
}

async function diffRPC(call: any, callback: any): Promise<void> {
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
        const provider = getProvider(news[providerKey] === runtime.unknownValue ? olds : news);
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

function diffConfigRPC(call: any, callback: any): void {
    callback({
        code: grpc.status.UNIMPLEMENTED,
        details: "DiffConfig is not implemented by the dynamic provider",
    }, undefined);
}

async function createRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.CreateResponse();

        const props = req.getProperties().toJavaScript();
        const provider = getProvider(props);
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

async function readRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.ReadResponse();

        const id = req.getId();
        const props = req.getProperties().toJavaScript();
        const provider = getProvider(props);
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

async function updateRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.UpdateResponse();

        const olds = req.getOlds().toJavaScript();
        const news = req.getNews().toJavaScript();

        let result: any = {};
        const provider = getProvider(news);
        if (provider.update) {
            result = await provider.update(req.getId(), olds, news) || {};
        }

        const resultProps = resultIncludingProvider(result.outs, news);
        resp.setProperties(structproto.Struct.fromJavaScript(resultProps));

        callback(undefined, resp);
    } catch (e) {
        const response = grpcResponseFromError(e);
        return callback(/*err:*/ response, /*value:*/ null, /*metadata:*/ response.metadata);
    }
}

async function deleteRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const props: any = req.getProperties().toJavaScript();
        const provider: any = await getProvider(props);
        if (provider.delete) {
            await provider.delete(req.getId(), props);
        }
        callback(undefined, new emptyproto.Empty());
    } catch (e) {
        console.error(`${e}: ${e.stack}`);
        callback(e, undefined);
    }
}

async function getPluginInfoRPC(call: any, callback: any): Promise<void> {
    const resp: any = new plugproto.PluginInfo();
    resp.setVersion(version);
    callback(undefined, resp);
}

function getSchemaRPC(call: any, callback: any): void {
    callback({
        code: grpc.status.UNIMPLEMENTED,
        details: "GetSchema is not implemented by the dynamic provider",
    }, undefined);
}

function constructRPC(call: any, callback: any): void {
    callback({
        code: grpc.status.UNIMPLEMENTED,
        details: "Construct is not implemented by the dynamic provider",
    }, undefined);
}

function resultIncludingProvider(result: any, props: any): any {
    return Object.assign(result || {}, {
        [providerKey]: props[providerKey],
    });
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

/** @internal */
export async function main(args: string[]) {
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
        "grpc.max_receive_message_length": maxRPCMessageSize,
    });
    server.addService(provrpc.ResourceProviderService, {
        cancel: cancelRPC,
        configure: configureRPC,
        invoke: invokeRPC,
        streamInvoke: streamInvokeRPC,
        check: checkRPC,
        checkConfig: checkConfigRPC,
        diff: diffRPC,
        diffConfig: diffConfigRPC,
        create: createRPC,
        read: readRPC,
        update: updateRPC,
        delete: deleteRPC,
        getPluginInfo: getPluginInfoRPC,
        getSchema: getSchemaRPC,
        construct: constructRPC,
    });
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

main(process.argv.slice(2));
