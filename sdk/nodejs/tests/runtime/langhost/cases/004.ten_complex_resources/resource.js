// Define and export a resource class that can be used by index.js.

let pulumi = require("../../../../../");

exports.MyResource = class MyResource extends pulumi.CustomResource {
    constructor(name, seq) {
        super("test:index:MyResource", name, {
            // First a few basic properties that are resolved to values.
            inseq: seq,
            inpropB1: false,
            inpropB2: true,
            inpropN: 42,
            inpropS: "a string",
            inpropA: [true, 99, "what a great property"],
            inpropO: {
                b1: false,
                b2: true,
                n: 42,
                s: "another string",
                a: [66, false, "strings galore"],
                o: { z: "x" },
            },

            // Next some properties that are completely unresolved (outputs).
            outseq: undefined,
            outprop1: undefined,
            outprop2: undefined,
        });
    }
};
