import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

// Stage 1: Change properties to trigger replacements
// Resource with deleteBeforeReplace option - should delete before creating
const withOption = new simple.Resource("withOption", {value: false}, {
    deleteBeforeReplace: true,
    replaceOnChanges: ["value"],
});
// Resource without deleteBeforeReplace - should create before deleting (default)
const withoutOption = new simple.Resource("withoutOption", {value: false}, {
    replaceOnChanges: ["value"],
});
