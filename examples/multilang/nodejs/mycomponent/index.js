const pulumi = require("@pulumi/pulumi");
const aws = require("@pulumi/aws");

// TODO: This should be done in the AWS library.  Also, not clear that providers work correctly
// currently - we appear to end up with class to `Check` a provider before it has been configured -
// I suspect that the proxy is getting confused as a separate identity even though it's the same
// URN.
pulumi.runtime.registerProxyConstructor("pulumi:providers:aws", aws.Provider)

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