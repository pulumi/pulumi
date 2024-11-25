// Test the dependsOn invoke option.

const assert = require("assert");
const pulumi = require("../../../../../");

const dependency = { resolved: false };

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

const dependsOn = pulumi.output(
    new Promise((resolve) =>
        setTimeout(() => {
            dependency.resolved = true;
            resolve(new MyResource("dependency"));
        }),
    ),
);

// By the time we serialise the arguments of the invoke, dependency.resolved
// should be true due to dependsOn awaiting the setTimeout promise resolution
// above. Without dependsOn, the invoke will be serialised before promise
// resolution (since promises [microtasks] happen before timeout events
// [macrotasks]).
pulumi.runtime.invokeOutput("test:index:echo", { dependency }, { dependsOn }).apply((result) => {
    assert.strictEqual(result.dependency.resolved, true);
});
