// This tests the creation of ten propertyless resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, args) {
        super("test:index:MyResource", name, args, {});
    }
}

let r0 = new MyResource("r0", {});
let r1 = new MyResource("r1", {});
let r2 = new MyResource("r2", {});

let o0 = new pulumi.Output(r0, Promise.resolve(42), Promise.resolve(true));
let o1 = new pulumi.Output(r1, Promise.resolve(24), Promise.resolve(true));
let o2 = new pulumi.Output(r2, Promise.resolve(99), Promise.resolve(true));

let r3 = new MyResource("r3", {
	v: o0.apply(_ => o1),
});

let r4 = new MyResource("r4", {
	v: o0.apply(_ => o1.apply(_ => o2)),
});
