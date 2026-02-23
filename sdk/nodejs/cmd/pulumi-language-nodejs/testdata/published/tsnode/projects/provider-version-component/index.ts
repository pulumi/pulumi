import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const withV22 = new conformance_component.Simple("withV22", {value: true});
const withDefault = new conformance_component.Simple("withDefault", {value: true});
// Ensure the simple plugin is discoverable for this conformance run.
const simpleResource = new simple.Resource("simpleResource", {value: false});
