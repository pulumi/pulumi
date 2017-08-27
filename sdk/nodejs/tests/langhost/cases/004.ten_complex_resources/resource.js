let fabric = require("../../../../lib");

exports.MyResource = class MyResource extends fabric.Resource {
    constructor(name, seq) {
        super("test:index:MyResource", name, {
            // First a few basic properties that are resolved to values.
            "inseq": new fabric.Property(seq),
            "inpropB1": new fabric.Property(false),
            "inpropB2": new fabric.Property(true),
            "inpropN": new fabric.Property(42),
            "inpropS": new fabric.Property("a string"),
            "inpropA": new fabric.Property([ true, 99, "what a great property" ]),
            "inpropO": new fabric.Property({
                b1: false,
                b2: true,
                n: 42,
                s: "another string",
                a: [ 66, false, "strings galore" ],
                o: { z: "x" },
            }),

            // Next some properties that are completely unresolved (outputs).
            "outseq": new fabric.Property(),
            "outprop1": new fabric.Property(),
            "outprop2": new fabric.Property(),
        });
    }
};

