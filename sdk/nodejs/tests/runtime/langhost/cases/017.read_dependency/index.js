// Test the ability to invoke provider functions via RPC.

const assert = require("assert");
const pulumi = require("../../../../../");

const inputs = {
    field: "state"
};

const res = new pulumi.CustomResource("custom:read:Read", "test", inputs, {
    id: "foobar"
});

const notRead = new pulumi.CustomResource("custom:read:Read", "not-read", {
    field: "testing",
});

const other = new pulumi.CustomResource("custom:read:Read", "dependent", {
    field: pulumi.all([res.field, notRead.field]).apply(([a, b]) => a + " " + b),
});

// Same thing as above, but using `dependsOn` instead of output dependencies
const dependsOnDep = new pulumi.CustomResource("custom:read:Read", "dependson", {
    field: "unrelated",
}, { dependsOn: [res, notRead]});

other.field.apply(v => assert.strictEqual(v, "foobar testing"));
dependsOnDep.field.apply(v => assert.strictEqual(v, "unrelated"));
