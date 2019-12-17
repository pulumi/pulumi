// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

class CustResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:CustResource", name, {}, opts)
    }
}

class CompResource extends pulumi.ComponentResource {
    constructor(name, opts) {
        super("test:index:CompResource", name, {}, opts)
        const data = this.getData();
        this.a = pulumi.output(data.then(d => d.a));
        this.b = pulumi.output(data.then(d => d.b));
    }

    /** @override */
    async initialize() {
        new CustResource("a", { parent: this });
        new CustResource("b", { parent: this });
        return { a: 1, b: 2 }
    }
}

const comp = new CompResource("comp", {});
comp.a.apply(v => {
    assert.equal(v, 1);
});
comp.b.apply(v => {
    assert.equal(v, 2);
});

// Have a custom resource depend on the async component.  We should still pick up 'a' and 'b' as
// dependencies.
const c = new CustResource("c", { dependsOn: comp });
