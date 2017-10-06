// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";

let fs = require("fs");
let grpc = require("grpc");
let emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
let structproto = require("google-protobuf/google/protobuf/struct_pb.js");
let provproto = require("../../proto/provider_pb.js");
let provrpc = require("../../proto/provider_grpc_pb.js");

const ResourcesFileName = "test-resources.json";
let resources: any;
let impl: any;

const URNPrefix: string = "urn:pulumi:";
const URNNameDelimiter: string = "::";
const NSDelimiter: string = ":";

function getTypeFromURN(urn: string): string {
	return urn.split(URNNameDelimiter)[2].split(NSDelimiter)[2];
}

function configureRPC(call: any, callback: any): void {
	let req = call.request;
	let resp = new emptyproto.Empty();

	let variables = req.getVariablesMap();
	let implJS = variables.get("test:provider:impl");
	impl = require(implJS);

	callback(undefined, resp);
}

function invokeRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.InvokeResponse();

	console.error(`invoke(${req})`);

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

	let type = getTypeFromURN(req.getUrn());
	let result: any = impl[type].diff(resources[req.getId()], req.getOlds().toJavaScript(), req.getNews().toJavaScript());

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
	console.error(ins);
	let result: any = impl[type].create(ins);

	let resource = result.resource;
	let id = Object.keys(resources).length;
	resources[id] = resource;

	fs.writeFileSync(ResourcesFileName, JSON.stringify(resources));

	resp.setId(id.toString());
	if (result.outs) {
		let properties: any = {};
		for (let k of result.outs) {
			properties[k] = resource[k];
		}
		console.error(properties);
		resp.setProperties(structproto.Struct.fromJavaScript(properties));
	}

	callback(undefined, resp);
}

function updateRPC(call: any, callback: any): void {
	let req: any = call.request;
	let resp = new provproto.UpdateResponse();

	let resource = resources[req.getId()];
	let type = getTypeFromURN(req.getUrn());
	let result: any = impl[type].update(resource, req.getOlds().toJavaScript(), req.getNews().toJavaScript());

	fs.writeFileSync(ResourcesFileName, JSON.stringify(resources));

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
	let resp = new emptyproto.Empty();

	let id = req.getId();
	let type = getTypeFromURN(req.getUrn());
	impl[type].delete(resources[id], req.getProperties());

	delete resources[id];

	fs.writeFileSync(ResourcesFileName, JSON.stringify(resources));

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
    let engineAddr: string = args[0];

	// Load up any serialized resource state.
	try {
		resources = JSON.parse(fs.readFileSync(ResourcesFileName));
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

