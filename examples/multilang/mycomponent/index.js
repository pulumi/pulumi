const pulumi = require("@pulumi/pulumi");

exports.MyComponent = class MyComponent extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("my:mod:MyComponent", name, {}, opts);
        this.output1 = pulumi.output(args.input1);
        this.myid = pulumi.output("foo");
        this.registerOutputs({
            myid: this.myid,
            output1: this.output1,
        });
    }
}
