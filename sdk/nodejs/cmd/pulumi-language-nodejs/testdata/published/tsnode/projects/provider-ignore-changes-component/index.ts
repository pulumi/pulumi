import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const withIgnoreChanges = new conformance_component.Simple("withIgnoreChanges", {value: true}, {
    ignoreChanges: ["value"],
});
const withoutIgnoreChanges = new conformance_component.Simple("withoutIgnoreChanges", {value: true});
// Make a simple resource so that plugin detection works.
const simpleResource = new simple.Resource("simpleResource", {value: false});
