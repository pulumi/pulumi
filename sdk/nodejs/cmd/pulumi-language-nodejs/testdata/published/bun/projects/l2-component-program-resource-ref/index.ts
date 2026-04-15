import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
const custom1 = new component.Custom("custom1", {value: component1.value});
const custom2 = new component.Custom("custom2", {value: component1.ref.value});
