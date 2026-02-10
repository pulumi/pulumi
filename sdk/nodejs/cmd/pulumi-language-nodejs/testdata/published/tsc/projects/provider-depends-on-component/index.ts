import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const noDependsOn = new conformance_component.Simple("noDependsOn", { value: true });
const withDependsOn = new conformance_component.Simple("withDependsOn", { value: true }, { dependsOn: [noDependsOn] });
// Make a simple resource so that plugin detection works.
const simpleResource = new simple.Resource("simpleResource", { value: false });
