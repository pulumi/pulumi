import * as pulumi from "@pulumi/pulumi";
import * as myext from "@pulumi/myext";

const greeting = new myext.Greeting("greeting", {});
const greetingComp = new myext.GreetingComponent("greetingComp", {});
export const parameterValue = greeting.parameterValue;
export const parameterValueFromComponent = greetingComp.parameterValue;
