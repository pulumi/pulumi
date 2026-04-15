import * as pulumi from "@pulumi/pulumi";
import * as snake_names from "@pulumi/snake_names";

// Resource inputs are correctly translated
const first = new snake_names.cool_module.Some_resource("first", {
    the_input: true,
    nested: {
        nested_value: "nested",
    },
});
// Datasource outputs are correctly translated
const third = new snake_names.cool_module.Another_resource("third", {the_input: snake_names.cool_module.some_dataOutput({
    the_input: first.the_output.someKey[0].nested_output,
    nested: [{
        value: "fuzz",
    }],
}).apply(invoke => invoke.nested_output[0].key.value)});
