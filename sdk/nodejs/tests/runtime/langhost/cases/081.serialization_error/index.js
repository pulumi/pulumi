// A resource whose input property fails to serialize: the `bad` property is an Output whose apply throws, so
// marshalling the inputs throws.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, props) {
        super("test:index:MyResource", name, props);
    }
}

new MyResource("testResource1", {
    bad: pulumi.output(1).apply(() => {
        throw new Error("💥 serialization goes boom");
    }),
});
