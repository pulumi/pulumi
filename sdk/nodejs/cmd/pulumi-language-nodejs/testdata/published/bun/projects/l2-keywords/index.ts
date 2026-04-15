import * as pulumi from "@pulumi/pulumi";
import * as keywords from "@pulumi/keywords";

const firstResource = new keywords.SomeResource("firstResource", {
    builtins: "builtins",
    lambda: "lambda",
    property: "property",
});
const secondResource = new keywords.SomeResource("secondResource", {
    builtins: firstResource.builtins,
    lambda: firstResource.lambda,
    property: firstResource.property,
});
const lambdaModuleResource = new keywords.lambda.SomeResource("lambdaModuleResource", {
    builtins: "builtins",
    lambda: "lambda",
    property: "property",
});
const lambdaResource = new keywords.Lambda("lambdaResource", {
    builtins: "builtins",
    lambda: "lambda",
    property: "property",
});
