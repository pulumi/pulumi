import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

// Component with no dependencies (the contrast)
const noDependsOn = new conformance_component.Simple("noDependsOn", {value: true});
// Component with dependsOn
const withDependsOn = new conformance_component.Simple("withDependsOn", {value: true}, {
    dependsOn: [noDependsOn],
});
// Make a simple resource so that plugin detection works.
const simpleResource = new simple.Resource("simpleResource", {value: false});
