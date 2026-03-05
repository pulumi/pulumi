import * as pulumi from "@pulumi/pulumi";
import * as component_property_deps from "@pulumi/component-property-deps";

const custom1 = new component_property_deps.Custom("custom1", {value: "hello"});
const custom2 = new component_property_deps.Custom("custom2", {value: "world"});
const component1 = new component_property_deps.Component("component1", {
    resource: custom1,
    resourceList: [
        custom1,
        custom2,
    ],
    resourceMap: {
        one: custom1,
        two: custom2,
    },
});
export const propertyDepsFromCall = component1.refs(({
    resource: custom1,
    resourceList: [
        custom1,
        custom2,
    ],
    resourceMap: {
        one: custom1,
        two: custom2,
    },
})).apply(call => call.result);
