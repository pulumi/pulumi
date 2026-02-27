import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const target = new conformance_component.Simple("target", {value: true});
const replaceWith = new conformance_component.Simple("replaceWith", {value: true}, {
    replaceWith: [target],
});
const notReplaceWith = new conformance_component.Simple("notReplaceWith", {value: true});
// Ensure the simple plugin is discoverable for this conformance run.
const simpleResource = new simple.Resource("simpleResource", {value: false});
