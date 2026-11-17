import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const config = new pulumi.Config();
const plainBool = config.requireBoolean("plainBool");
const plainNumber = config.requireNumber("plainNumber");
const plainInteger = config.requireNumber("plainInteger");
const plainString = config.require("plainString");
const plainNumericString = config.require("plainNumericString");
const secretNumber = config.requireSecretNumber("secretNumber");
const secretInteger = config.requireSecretNumber("secretInteger");
const secretString = config.requireSecret("secretString");
const secretNumericString = config.requireSecret("secretNumericString");
const plainValues = new primitive.Resource("plainValues", {
    boolean: plainString === "true",
    float: plainInteger,
    integer: Number(plainNumericString),
    string: String(plainNumber),
    numberArray: [
        plainInteger,
        Number(plainNumericString),
        plainNumber,
    ],
    booleanMap: {
        fromBool: plainBool,
        fromString: plainString === "true",
    },
});
const secretValues = new primitive.Resource("secretValues", {
    boolean: secretString.apply(x =>x === "true"),
    float: secretInteger,
    integer: secretNumericString.apply(x =>Number(x)),
    string: secretNumber.apply(x =>String(x)),
    numberArray: [
        plainInteger,
        Number(plainNumericString),
        plainNumber,
    ],
    booleanMap: {
        fromBool: plainBool,
        fromString: plainString === "true",
    },
});
const invokeResult = primitive.invokeOutput({
    boolean: plainString === "true",
    float: plainInteger,
    integer: Number(plainNumericString),
    string: String(plainBool),
    numberArray: [
        plainInteger,
        Number(plainNumericString),
        plainNumber,
    ],
    booleanMap: {
        fromBool: plainBool,
        fromString: plainString === "true",
    },
});
const invokeValues = new primitive.Resource("invokeValues", {
    boolean: invokeResult.boolean,
    float: invokeResult.float,
    integer: invokeResult.integer,
    string: invokeResult.string,
    numberArray: invokeResult.numberArray,
    booleanMap: invokeResult.booleanMap,
});
