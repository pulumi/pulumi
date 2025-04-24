// This tests that resources cannot depend on things which are not resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, deps) {
        super("test:index:MyResource", name, {}, deps);
    }
}

new MyResource("testResource", { dependsOn: pulumi.output(Promise.resolve([Promise.resolve(1)])) });
