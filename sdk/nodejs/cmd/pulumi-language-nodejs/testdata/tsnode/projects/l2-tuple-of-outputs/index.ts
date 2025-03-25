import * as pulumi from "@pulumi/pulumi";
import * as component_property_deps from "@pulumi/component-property-deps";

// An output that will be used to verify tuple construction works correctly.
const foo = pulumi.secret({
    superSecret: true,
});
const bar = pulumi.secret({
    superSecret: false,
});
// These resources are just to satisfy the input requirements of the components.
const custom1 = new component_property_deps.Custom("custom1", {value: "hello"});
const custom2 = new component_property_deps.Custom("custom2", {value: "world"});
const component1 = new component_property_deps.Component("component1", {
    resource: custom1,
    resourceList: [foo.superSecret],
    resourceMap: {
        one: custom1,
        two: custom2,
    },
});
const component2 = new component_property_deps.Component("component2", {
    resource: custom1,
    resourceList: [
        foo.superSecret,
        bar.superSecret,
    ],
    resourceMap: {
        one: custom1,
        two: custom2,
    },
});
