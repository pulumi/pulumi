const pulumi = require("@pulumi/pulumi");

class MyCustomResource extends pulumi.dynamic.Resource {
    constructor(name, args, opts) {
        const provider = {
            async create(inputs) {
                return {
                    id: "abcd",
                    outs: { in1: inputs.in1, out1: inputs.in1 + 1 },
                };
            },

        }
        super(provider, name, args, opts);
    }
}
exports.MyCustomResource = MyCustomResource;

class MyInnerComponent extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("my:mod:MyInnerComponent", name, {}, opts);
        this.data = "mydata";
        this.registerOutputs({
            data: "mydata",
        });
    }
}

class MyComponent extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("my:mod:MyComponent", name, {}, opts);
        this.output1 = pulumi.output(args.input1);
        this.myid = pulumi.output("foo");
        this.customResource = new MyCustomResource("custom", { in1: args.input1 }, { parent: this });
        this.innerComponent = new MyInnerComponent("inner", { parent: this });
        this.registerOutputs({
            myid: this.myid,
            output1: this.output1,
            customResource: this.customResource,
            innerComponent: this.innerComponent,
        });
    }
}
exports.MyComponent = MyComponent;