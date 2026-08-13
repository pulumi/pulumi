import * as pulumi from "@pulumi/pulumi";
import * as kebab_names from "@pulumi/kebab-names";

// The package name and module name are kebab-case. Resource and object type names cannot be
// kebab-case yet (the metaschema forbids hyphens in the member segment of a token), and kebab-case
// property names are not yet handled by all code generators.
const first = new kebab_names.kebab_module.SomeResource("first", {
    theInput: true,
    nested: {
        nestedValue: "nested",
    },
});
const second = new kebab_names.kebab_module.AnotherResource("second", {theInput: first.theOutput.nestedOutput});
