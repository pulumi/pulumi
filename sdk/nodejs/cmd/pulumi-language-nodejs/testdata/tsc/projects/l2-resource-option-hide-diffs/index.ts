import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const hideDiffs = new simple.Resource("hideDiffs", {value: true}, {
    hideDiffs: ["value"],
});
const notHideDiffs = new simple.Resource("notHideDiffs", {value: true});
