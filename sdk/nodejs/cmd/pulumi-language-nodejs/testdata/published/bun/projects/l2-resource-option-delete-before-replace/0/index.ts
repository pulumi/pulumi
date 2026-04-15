import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

// Stage 0: Initial resource creation
// Resource with deleteBeforeReplace option
const withOption = new simple.Resource("withOption", {value: true}, {
    deleteBeforeReplace: true,
    replaceOnChanges: ["value"],
});
// Resource without deleteBeforeReplace (default create-before-delete behavior)
const withoutOption = new simple.Resource("withoutOption", {value: true}, {
    replaceOnChanges: ["value"],
});
