// Test the dependsOn invoke option.

const assert = require("assert");
const pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

const dependsOn = pulumi.output(
    new Promise((resolve) =>
        setTimeout(() => {
            resolve(new MyResource("dependency"));
        }),
    ),
);

pulumi.runtime.invoke("test:index:echo", {}, { dependsOn });
