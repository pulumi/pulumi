// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is a mock resource provider that can be used to implement custom CRUD operations in JavaScript.
// It is configured using a single variable, `test:provider:crud`, that provides the path to the JS module
// that implements CRUD operations for various types. When an operation is requested by the engine for a
// resource of a particular type, that type's `CRUDProvider` is loaded from the input module using the
// unqualified type name.

import * as minimist from "minimist";

let path = require("path");
let grpc = require("grpc");
let emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
let structproto = require("google-protobuf/google/protobuf/struct_pb.js");
let provproto = require("../../proto/provider_pb.js");
let provrpc = require("../../proto/provider_grpc_pb.js");

interface CRUDProvider {
    check(ins: any): any;
    diff(id: string, olds: any, news: any): any;
    create(inputs: any): any;
    update(id: string, olds: any, news: any): any;
    delete(id: string, props: any): void;
}

class CRUDProviders {
    private crud: any;

    constructor(crud: any) {
        this.crud = crud;
    }

    getProvider(urn: string): CRUDProvider {
        const URNNameDelimiter: string = "::";
        const NSDelimiter: string = ":";

        const type = urn.split(URNNameDelimiter)[2].split(NSDelimiter)[2];
        return this.crud[type];
    }
}
let crud: CRUDProviders;

function configureRPC(call: any, callback: any): void {
    const req = call.request;

    const variables = req.getVariablesMap();
    let crudJS = variables.get("test:provider:crud");
    if (crudJS.startsWith("./") || crudJS.startsWith("../")) {
        crudJS = path.normalize(path.join(process.cwd(), crudJS));
    }
    crud = new CRUDProviders(require(crudJS));

    callback(undefined, new emptyproto.Empty());
}

function invokeRPC(call: any, callback: any): void {
    const req: any = call.request;
    const resp = new provproto.InvokeResponse();

    // TODO: implement this.

    callback(undefined, resp);
}

function checkRPC(call: any, callback: any): void {
    const req: any = call.request;
    const resp = new provproto.CheckResponse();

    const result = crud.getProvider(req.getUrn()).check(req.getProperties().toJavaScript());
    if (result.defaults) {
        resp.setDefaults(structproto.Struct.fromJavaScript(result.defaults));
    }
    if (result.failures) {
        const failures = [];
        for (const f of result.failures) {
            const failure = new provproto.CheckFailure();
            failure.setProperty(f.property);
            failure.setReason(f.reason);

            failures.push(failure);
        }
        resp.setFailuresList(failures);
    }

    callback(undefined, resp);
}

function diffRPC(call: any, callback: any): void {
    const req: any = call.request;
    const resp = new provproto.DiffResponse();

    const result: any = crud.getProvider(req.getUrn()).diff(req.getId(), req.getOlds().toJavaScript(), req.getNews().toJavaScript());
    if (result.replaces) {
        resp.setReplaces(result.replaces);
    }

    callback(undefined, resp);
}

function createRPC(call: any, callback: any): void {
    const req: any = call.request;
    const resp = new provproto.CreateResponse();

    const result: any = crud.getProvider(req.getUrn()).create(req.getProperties().toJavaScript());

    const resource = result.resource;
    resp.setId(result.id.toString());
    if (result.outs) {
        const properties: any = {};
        for (const k of result.outs) {
            properties[k] = resource[k];
        }
        resp.setProperties(structproto.Struct.fromJavaScript(properties));
    }

    callback(undefined, resp);
}

function updateRPC(call: any, callback: any): void {
    const req: any = call.request;
    const resp = new provproto.UpdateResponse();

    const result: any = crud.getProvider(req.getUrn()).update(req.getId(), req.getOlds().toJavaScript(), req.getNews().toJavaScript());

    const resource = result.resource;
    if (result.outs) {
        const properties: any = {};
        for (const k of result.outs) {
            properties[k] = resource[k];
        }
        resp.setProperties(structproto.Struct.fromJavaScript(properties));
    }

    callback(undefined, resp);
}

function deleteRPC(call: any, callback: any): void {
    const req: any = call.request;
    crud.getProvider(req.getUrn()).delete(req.getId(), req.getProperties());
    callback(undefined, new emptyproto.Empty());
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
        update: updateRPC,
        delete: deleteRPC
    });
    const port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

    server.start();

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}

main(process.argv.slice(2));
