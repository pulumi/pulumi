import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const provider = new simple.Provider("provider", {});
const parent1 = new simple.Resource("parent1", {value: true}, {
    provider: provider,
});
const child1 = new simple.Resource("child1", {value: true}, {
    parent: parent1,
});
const orphan1 = new simple.Resource("orphan1", {value: true});
const parent2 = new simple.Resource("parent2", {value: true}, {
    protect: true,
});
const child2 = new simple.Resource("child2", {value: true}, {
    parent: parent2,
});
const child3 = new simple.Resource("child3", {value: true}, {
    parent: parent2,
    protect: false,
});
const orphan2 = new simple.Resource("orphan2", {value: true});
