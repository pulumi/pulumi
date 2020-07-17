// This tests the creation of a resource that contains a very large string.
// In particular we are testing sending large message sizes (>4mb) over an RPC call.

let pulumi = require("../../../../../");

// Read the file contents to create a very large string (5mb)
const longString = 'a'.repeat(1024 * 1024 * 5)

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyLargeStringResource", name, {
            "largeStringProp": longString,
        });
    }
}

new MyResource("testResource1");
