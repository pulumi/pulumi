let assert = require("assert");
let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
	constructor(name, args, opts) {
		super("test:index:MyResource", name, args, opts);
	}
}

//            cust1
//            /   \
//       cust2    cust3

let cust1 = new MyResource("cust1");
let cust2 = new MyResource("cust2", { parentId: cust1.id }, { parent: cust1 });
let cust3 = new MyResource("cust3", { parentId: cust1.id }, { parent: cust1 });
