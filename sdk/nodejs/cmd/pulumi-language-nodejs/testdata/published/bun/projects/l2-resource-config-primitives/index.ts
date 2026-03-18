import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const config = new pulumi.Config();
const plainBool = config.requireBoolean("plainBool");
const plainNumber = config.requireNumber("plainNumber");
const plainString = config.require("plainString");
const secretBool = config.requireSecretBoolean("secretBool");
const secretNumber = config.requireSecretNumber("secretNumber");
const secretString = config.requireSecret("secretString");
const plain = new primitive.Resource("plain", {
    boolean: plainBool,
    float: plainNumber + 0.5,
    integer: plainNumber,
    string: plainString,
    numberArray: [
        -1,
        0,
        1,
    ],
    booleanMap: {
        t: true,
        f: false,
    },
});
const secret = new primitive.Resource("secret", {
    boolean: secretBool,
    float: secretNumber.apply(secretNumber => secretNumber + 0.5),
    integer: secretNumber,
    string: secretString,
    numberArray: [
        -2,
        0,
        2,
    ],
    booleanMap: {
        t: true,
        f: false,
    },
});
