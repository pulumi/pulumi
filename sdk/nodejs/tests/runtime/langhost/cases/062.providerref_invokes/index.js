// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

(async () => {
    class Provider extends pulumi.ProviderResource {
        constructor(name, opts) {
            super("test", name, {}, opts);
        }
    }

    const provider = await pulumi.ProviderRef.get(new Provider("p"));

    let args = {
        a: "hello",
        b: true,
        c: [0.99, 42, { z: "x" }],
        id: "some-id",
        urn: "some-urn",
    };

    let result1 = pulumi.runtime.invoke("test:index:echo", args, { provider });
    for (const key in args) {
        assert.deepEqual(result1[key], args[key]);
    }

    let result2 = pulumi.runtime.invoke("test:index:echo", args, { provider });
    result2.then((v) => {
        assert.deepEqual(v, args);
    });

    let result3 = pulumi.runtime.invoke("test:index:echo", args, { provider, async: true });
    result3.then((v) => {
        assert.deepEqual(v, args);
    });
})();