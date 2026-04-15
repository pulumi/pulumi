import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

const explicit = new component.Provider("explicit", {});
const list = new component.ComponentCallable("list", {value: "value"}, {
    providers: [explicit],
});
const map = new component.ComponentCallable("map", {value: "value"}, {
    providers: {
        component: explicit,
    },
});
