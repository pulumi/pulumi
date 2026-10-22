import * as pulumi from "@pulumi/pulumi";
import * as failingcomponent from "@pulumi/failingcomponent";
import * as simple from "@pulumi/simple";

const comp = new failingcomponent.Component("comp", {});
const dependent = new simple.Resource("dependent", {value: comp.value.apply(value => value == "hello")});
const independent = new simple.Resource("independent", {value: true});
export const componentValue = comp.value;
