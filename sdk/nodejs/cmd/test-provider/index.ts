// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";

let path = require("path");
let grpc = require("grpc");
let emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
let structproto = require("google-protobuf/google/protobuf/struct_pb.js");
let provproto = require("../../proto/provider_pb.js");
let provrpc = require("../../proto/provider_grpc_pb.js");

let crud: any;

const URNPrefix: string = "urn:pulumi:";
const URNNameDelimiter: string = "::";
const NSDelimiter: string = ":";

function getTypeFromURN(urn: string): string {
	return urn.split(URNNameDelimiter)[2].split(NSDelimiter)[2];
}

function configureRPC(call: any, callback: any): void {
	let req = call.request;

	let variables = req.getVariablesMap();
	let crudJS = variables.get("test:provider:crud");
    if (crudJS.startsWith("./") || crudJS.startsWith("../")) {
        crudJS = path.normalize(path.join(process.cwd(), crudJS));
    }
	crud = require(crudJS);

	callback(undefined, new emptyproto.Empty());
}

function invokeRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.InvokeResponse();

	// TODO: implement this.

	callback(undefined, resp);
}

function checkRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.CheckResponse();

	let type = getTypeFromURN(req.getUrn());
	let result: any = crud[type].check(req.getProperties().toJavaScript());

	if (result.defaults) {
		resp.setDefaults(structproto.Struct.fromJavaScript(result.defaults));
	}
	if (result.failures) {
		let failures = [];
		for (let f of result.failures) {
			let failure = new provproto.CheckFailure();
			failure.setProperty(f.property);
			failure.setReason(f.reason);

			failures.push(failure);
		}
		resp.setFailuresList(failures);
	}

	callback(undefined, resp);
}

function diffRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.DiffResponse();

	let type = getTypeFromURN(req.getUrn());
	let result: any = crud[type].diff(req.getId(), req.getOlds().toJavaScript(), req.getNews().toJavaScript());

	if (result.replaces) {
		resp.setReplaces(result.replaces);
	}

	callback(undefined, resp);
}

function createRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.CreateResponse();

	let type = getTypeFromURN(req.getUrn());
	let ins = req.getProperties().toJavaScript();
	let result: any = crud[type].create(ins);

	let resource = result.resource;
	resp.setId(result.id.toString());
	if (result.outs) {
		let properties: any = {};
		for (let k of result.outs) {
			properties[k] = resource[k];
		}
		resp.setProperties(structproto.Struct.fromJavaScript(properties));
	}

	callback(undefined, resp);
}

function updateRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.UpdateResponse();

	let type = getTypeFromURN(req.getUrn());
	let result: any = crud[type].update(req.getId(), req.getOlds().toJavaScript(), req.getNews().toJavaScript());

	let resource = result.resource;
	if (result.outs) {
		let properties: any = {};
		for (let k of result.outs) {
			properties[k] = resource[k];
		}
		resp.setProperties(structproto.Struct.fromJavaScript(properties));
	}

	callback(undefined, resp);
}

function deleteRPC(call: any, callback: any): void {
	let req: any = call.request;

	let type = getTypeFromURN(req.getUrn());
	crud[type].delete(req.getId(), req.getProperties());

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
    let engineAddr: string = args[0];

    // Finally connect up the gRPC client/server and listen for incoming requests.
	let server = new grpc.Server();
	server.addService(provrpc.ResourceProviderService, {
		configure: configureRPC,
		invoke: invokeRPC,
		check: checkRPC,
		diff: diffRPC,
		create: createRPC,
		update: updateRPC,
		delete: deleteRPC
	});
	let port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

	server.start();

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}

main(process.argv.slice(2));
