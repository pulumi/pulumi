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
        this.cust1 = pulumi.output(data.then(d => d.cust1));
        this.cust2 = pulumi.output(data.then(d => d.cust2));
    }

    /** @override */
    async initialize() {
        const cust1 = new CustResource("a", { parent: this });
        const cust2 = new CustResource("b", { parent: this });
        return { a: 1, b: 2, cust1, cust2 }
    }
}

const comp = new CompResource("comp", {});
comp.a.apply(v => {
    assert.strictEqual(v, 1);
});
comp.b.apply(v => {
    assert.strictEqual(v, 2);
});

// Have a custom resource depend on the async component.  We should still pick up 'a' and 'b' as
// dependencies.
const c = new CustResource("c", { dependsOn: comp });

// Have another depend on the child resources that are exposed through Output wrappers of async
// computation.   We should still pick up 'a' and 'b' as dependencies.
const d = new CustResource("d", { dependsOn: [comp.cust1, comp.cust2] });
