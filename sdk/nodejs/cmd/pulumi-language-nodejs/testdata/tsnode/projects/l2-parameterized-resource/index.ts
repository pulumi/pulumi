import * as pulumi from "@pulumi/pulumi";
import * as subpackage from "@pulumi/subpackage";

// The resource name is based on the parameter value
const example = new subpackage.HelloWorld("example", {});
export const parameterValue = example.parameterValue;
