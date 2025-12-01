import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
const component2 = new component.ComponentCustomRefInputOutput("component2", {inputRef: component1.ref});
const custom1 = new component.Custom("custom1", {value: component2.inputRef.value});
const custom2 = new component.Custom("custom2", {value: component2.outputRef.value});
