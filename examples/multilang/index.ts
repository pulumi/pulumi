import * as aws from "@pulumi/aws";
import * as pulumi from "@pulumi/pulumi";
import * as mycomponent from "./proxy";

// This should go inside `@pulumi/aws`.
pulumi.runtime.registerProxyConstructor("aws:ec2/securityGroup:SecurityGroup", aws.ec2.SecurityGroup);

////////////////////////////////
// This is code the user would write to use `mycomponent` from the guest language.

const res = new mycomponent.MyComponent("n", {
    input1: Promise.resolve(42),
}, { ignoreChanges: ["input1"] /*, providers: { "aws": awsProvider } */ });

export const id2 = res.myid;
export const output1 = res.output1;
export const innerComponent = res.innerComponent.data;
export const nodeSecurityGroupId = res.nodeSecurityGroup.id;
