import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const res = new conformance_component.Simple("res", {value: true}, {
    replacementTrigger: "trigger-value-updated",
});
const simpleResource = new simple.Resource("simpleResource", {value: false});
