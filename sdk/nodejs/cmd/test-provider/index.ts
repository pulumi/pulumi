// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";

let fs = require("fs");
let grpc = require("grpc");
let provproto = require("../../proto/provider_pb.js");
let provrpc = require("../../proto/provider_grpc_pb.js");

let resources: any;
let impl: any;

const URNPrefix: string = "urn:pulumi:";
const URNNameDelimiter: string = "::";

function getTypeFromURN(urn: string): string {
	return urn.split(URNNameDelimiter)[2];
}

function configureRPC(call: any, callback: any): void {
	let req = call.request;

	impl = require(req.variables.impl);

	callback(undefined, undefined);
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

	// TODO: implement this.

	callback(undefined, resp);
}

function diffRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.DiffResponse();

	let type = getTypeFromURN(req.urn);
	let result: any = impl[type].diff(resources[req.id], req.olds, req.news);

	resp.replaces = result.replaces;

	callback(undefined, resp);
}

function createRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.CreateResponse();

	let type = getTypeFromURN(req.urn);
	let result: any = impl[type].create(req.properties);

	let id = Object.keys(resources).length - 1;
	resources[id] = result.resource;

	resp.id = id;
	resp.properties = result.outs;

	callback(undefined, resp);
}

function updateRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.UpdateResponse();

	let type = getTypeFromURN(req.urn);
	let result: any = impl[type].udpate(resources[req.id], req.olds, req.news);

	resp.properties = result.properties;

	callback(undefined, resp);
}

function deleteRPC(call: any, callback: any): void {
	let req: any = call.request;

	let type = getTypeFromURN(req.urn);
	impl[type].delete(resources[req.id], req.properties);

	delete resources[req.id];

	callback(undefined, undefined);
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

	// Load up any serialized resource state.
	try {
		resources = JSON.parse(fs.readFileSync("test-provider-resources.json"));
	} catch (e) {
		console.error(`fatal: could not load resources: ${e}`);
		process.exit(-1);
		return;
	}

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

