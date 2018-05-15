// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as minimist from "minimist";
import * as path from "path";

import * as dynamic from "../../dynamic";
import * as resource from "../../resource";
import { version } from "../../version";

const requireFromString = require("require-from-string");
const grpc = require("grpc");
const emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");
const provproto = require("../../proto/provider_pb.js");
const provrpc = require("../../proto/provider_grpc_pb.js");
const plugproto = require("../../proto/plugin_pb.js");

const providerKey: string = "__provider";

function getProvider(props: any): dynamic.ResourceProvider {
    // TODO[pulumi/pulumi#414]: investigate replacing requireFromString with eval
    return requireFromString(props[providerKey]).handler();
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

function configureRPC(call: any, callback: any): void {
    callback(undefined, new emptyproto.Empty());
}

async function invokeRPC(call: any, callback: any): Promise<void> {
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
        const provider = getProvider(news);

        let inputs: any = {};
        let failures: any[] = [];
        if (provider.check) {
            const result = await provider.check(olds, news);
            if (result.inputs) {
                inputs = result.inputs;
            }
            if (result.failures) {
                failures = result.failures;
            }
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

async function diffRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.DiffResponse();

        // If the provider itself has changed, do not delegate to the dynamic provider. Instead, simply report that the
        // resource requires replacement. This allows the new resource to be created using the new provider and the old
        // resource to be deleted using the old provider.
        const olds = req.getOlds().toJavaScript();
        const news = req.getNews().toJavaScript();
        if (olds[providerKey] !== news[providerKey]) {
            resp.setReplacesList([ providerKey ]);
        } else {
            const provider = getProvider(olds);
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
                    resp.setDeleteBeforeReplace(result.deleteBeforeReplace);
                }
            }
        }

        callback(undefined, resp);
    } catch (e) {
        console.error(`${e}: ${e.stack}`);
        callback(e, undefined);
    }
}

async function createRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.CreateResponse();

        const props = req.getProperties().toJavaScript();
        const provider = getProvider(props);

        const result = await provider.create(props);
        resp.setId(result.id);
        if (result.outs) {
            resp.setProperties(structproto.Struct.fromJavaScript(result.outs));
        }

        if (result.error) {
            resp.setError(result.error);
        }

        callback(undefined, resp);
    } catch (e) {
        console.error(`${e}: ${e.stack}`);
        callback(e, undefined);
    }
}

async function readRPC(call: any, callback: any): Promise<void> {
    try {
        const req: any = call.request;
        const resp = new provproto.ReadResponse();

        const props = req.getProperties().toJavaScript();
        const provider = getProvider(props);
        if (provider.read) {
            const result: any = await provider.read(req.getId(), props);
            if (result.properties) {
                resp.setProperties(structproto.Struct.fromJavaScript(result.properties));
            }
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
        if (olds[providerKey] !== news[providerKey]) {
            throw new Error("changes to provider should require replacement");
        }

        const provider = getProvider(olds);
        if (provider.update) {
            const result: any = await provider.update(req.getId(), olds, news);
            if (result.outs) {
                resp.setProperties(structproto.Struct.fromJavaScript(result.outs));
            }

            if (result.error) {
                resp.setError(result.error);
            }
        }

        callback(undefined, resp);
    } catch (e) {
        console.error(`${e}: ${e.stack}`);
        callback(e, undefined);
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

export function main(args: string[]): void {
    // The program requires a single argument: the address of the RPC endpoint for the engine.  It
    // optionally also takes a second argument, a reference back to the engine, but this may be missing.
    if (args.length === 0) {
        console.error("fatal: Missing <engine> address");
        process.exit(-1);
        return;
    }
    const engineAddr: string = args[0];

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const server = new grpc.Server();
    server.addService(provrpc.ResourceProviderService, {
        configure: configureRPC,
        invoke: invokeRPC,
        check: checkRPC,
        diff: diffRPC,
        create: createRPC,
        read: readRPC,
        update: updateRPC,
        delete: deleteRPC,
        getPluginInfo: getPluginInfoRPC,
    });
    const port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

    server.start();

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}

main(process.argv.slice(2));
