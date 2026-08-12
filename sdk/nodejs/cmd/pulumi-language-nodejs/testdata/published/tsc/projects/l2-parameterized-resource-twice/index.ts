import * as pulumi from "@pulumi/pulumi";
import * as byepackage from "@pulumi/byepackage";
import * as hipackage from "@pulumi/hipackage";

// The resource name is based on the parameter value
const example1 = new hipackage.HelloWorld("example1", {});
const exampleComponent1 = new hipackage.HelloWorldComponent("exampleComponent1", {});
export const parameterValue1 = example1.parameterValue;
export const parameterValueFromComponent1 = exampleComponent1.parameterValue;
// The resource name is based on the parameter value
const example2 = new byepackage.GoodbyeWorld("example2", {});
const exampleComponent2 = new byepackage.GoodbyeWorldComponent("exampleComponent2", {});
export const parameterValue2 = example2.parameterValue;
export const parameterValueFromComponent2 = exampleComponent2.parameterValue;
