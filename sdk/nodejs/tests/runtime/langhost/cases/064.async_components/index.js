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

        // Kick off async work that actually makes our children.
        this.asyncInitialize().then(_ => this.registerOutputs());
    }

    async asyncInitialize() {
        new CustResource("a", { parent: this });
        new CustResource("b", { parent: this });
    }

    isAsyncConstructed() {
        return true;
    }
}

const comp = new CompResource("comp", {});

// Have a custom resource depend on the async component.  We should still pick up 'a' and 'b' as
// dependencies.
const c = new CustResource("c", { dependsOn: comp });
