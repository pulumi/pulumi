import * as pulumi from "@pulumi/pulumi";
import * as camelNames from "@pulumi/camelNames";

const firstResource = new camelNames.coolmodule.SomeResource("firstResource", {theInput: true});
const secondResource = new camelNames.coolmodule.SomeResource("secondResource", {theInput: firstResource.theOutput});
const thirdResource = new camelNames.coolmodule.SomeResource("thirdResource", {
    theInput: true,
    resourceName: "my-cluster",
});
