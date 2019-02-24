// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
	constructor(name, args, opts) {
		super("test:index:MyResource", name, args, opts);
	}
}

let resA = new MyResource("resA", {});
let resB = new MyResource("resB", { parentId: resA.id }, { parent: resA });
