import * as pulumi from "@pulumi/pulumi";
import * as kebab_names from "@pulumi/kebab-names";

// The package name, module name, resource names, object type names and property names are all
// kebab-case.
const first = new kebab_names.kebab_module.SomeResource("first", {
    "the-input": true,
    nested: {
        "nested-value": "nested",
    },
});
const second = new kebab_names.kebab_module.AnotherResource("second", {"the-input": first["the-output"]["nested-output"]});
export const theOutput = first["the-output"];
