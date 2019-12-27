const pulumi = require("@pulumi/pulumi");
const aws = require("@pulumi/aws");

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
        this.innerComponent = new MyInnerComponent("inner", {}, { parent: this });
        this.nodeSecurityGroup = new aws.ec2.SecurityGroup("securityGroup", {}, { parent: this });
        this.registerOutputs({
            myid: this.myid,
            output1: this.output1,
            innerComponent: this.innerComponent,
            nodeSecurityGroup: this.nodeSecurityGroup,
        });
    }
}
exports.MyComponent = MyComponent;