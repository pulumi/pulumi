import * as pulumi from "@pulumi/pulumi";
import * as kebab_names from "@pulumi/kebab-names";

// The package name, module name and property names are kebab-case. Resource and object type names
// cannot be kebab-case yet: the metaschema forbids hyphens in the member segment of a token.
const first = new kebab_names.kebab_module.SomeResource("first", {
    "the-input": true,
    nested: {
        "nested-value": "nested",
    },
});
const second = new kebab_names.kebab_module.AnotherResource("second", {"the-input": first["the-output"]["nested-output"]});
