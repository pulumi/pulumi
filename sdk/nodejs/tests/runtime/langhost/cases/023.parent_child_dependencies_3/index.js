// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

class MyCustomResource extends pulumi.CustomResource {
	constructor(name, args, opts) {
		super("test:index:MyCustomResource", name, args, opts);
	}
}

class MyComponentResource extends pulumi.ComponentResource {
	constructor(name, args, opts) {
		super("test:index:MyComponentResource", name, args, opts);
	}
}

let resA = new MyComponentResource("resA");
let resB = new MyCustomResource("resB", { parentId: resA.urn }, { parent: resA });
let resC = new MyCustomResource("resC", { parentId: resA.urn }, { parent: resA });
