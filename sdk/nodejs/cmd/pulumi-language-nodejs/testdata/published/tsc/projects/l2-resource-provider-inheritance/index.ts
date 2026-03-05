import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";
import * as simple from "@pulumi/simple";

const provider = new simple.Provider("provider", {});
const parent1 = new simple.Resource("parent1", {value: true}, {
    provider: provider,
});
// This should inherit the explicit provider from parent1
const child1 = new simple.Resource("child1", {value: true}, {
    parent: parent1,
});
const parent2 = new primitive.Resource("parent2", {
    boolean: false,
    float: 0,
    integer: 0,
    string: "",
    numberArray: [],
    booleanMap: {},
});
// This _should not_ inherit the provider from parent2 as it is a default provider.
const child2 = new simple.Resource("child2", {value: true}, {
    parent: parent2,
});
// This _should not_ inherit the provider from parent1 as its from the wrong package.
const child3 = new primitive.Resource("child3", {
    boolean: false,
    float: 0,
    integer: 0,
    string: "",
    numberArray: [],
    booleanMap: {},
}, {
    parent: parent1,
});
