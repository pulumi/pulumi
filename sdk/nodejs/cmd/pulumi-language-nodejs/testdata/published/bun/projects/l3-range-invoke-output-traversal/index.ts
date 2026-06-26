import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

// A resource whose computed output feeds the invoke, forcing the invoke into its
// output-versioned form so that `values` is an Output.
const source = new nestedobject.Container("source", {inputs: [
    "alpha",
    "bravo",
    "charlie",
]});
const values = nestedobject.getValuesOutput({
    names: source.inputs,
});
// Ranges over the length of the invoke's computed list and indexes that same
// Output-typed list by the loop counter inside the body. This is the shape from
// https://github.com/pulumi/pulumi/issues/12507.
const routes: nestedobject.Target[] = [];
values.results.length.apply(rangeBody => {
    for (let range = 0; range < rangeBody; range++) {
        routes.push(new nestedobject.Target(`routes-${range}`, {name: values.apply(values => values.results[range])}));
    }
});
