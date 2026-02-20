import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const withReplaceOnChanges = new conformance_component.Simple("withReplaceOnChanges", {value: true}, {
    replaceOnChanges: ["value"],
});
const withoutReplaceOnChanges = new conformance_component.Simple("withoutReplaceOnChanges", {value: true});
// Make a simple resource so that plugin detection works.
const simpleResource = new simple.Resource("simpleResource", {value: false});
