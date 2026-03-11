import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

// Make a simple resource to use as a parent
const parent = new simple.Resource("parent", {value: true});
// parent "res" to a new parent and alias it so it doesn't recreate.
const res = new conformance_component.Simple("res", {value: true}, {
    aliases:[{parent: (true ? pulumi.rootStackResource : undefined)}],
    parent: parent,
});
// Make a simple resource so that plugin detection works.
const simpleResource = new simple.Resource("simpleResource", {value: false});
