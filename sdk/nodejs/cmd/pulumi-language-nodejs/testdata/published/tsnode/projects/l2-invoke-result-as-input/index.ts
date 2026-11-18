import * as pulumi from "@pulumi/pulumi";
import * as invokeresult from "@pulumi/invokeresult";

const first = invokeresult.makeSettingsOutput({
    displayName: "first",
    maxItems: 1,
});
const second = invokeresult.makeSettingsOutput({
    displayName: "second",
    maxItems: 2,
});
const list = invokeresult.listOfOutput({
    items: [
        first,
        second,
    ],
});
const holder = new invokeresult.Holder("holder", {
    settings: first,
    all: list.apply(list => list.items),
});
