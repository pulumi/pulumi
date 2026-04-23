import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const config = new pulumi.Config();
const plainBool = config.requireBoolean("plainBool");
const plainNumber = config.requireNumber("plainNumber");
const plainInteger = config.requireNumber("plainInteger");
const plainString = config.require("plainString");
const secretBool = config.requireSecretBoolean("secretBool");
const secretNumber = config.requireSecretNumber("secretNumber");
const secretInteger = config.requireSecretNumber("secretInteger");
const secretString = config.requireSecret("secretString");
const plain = new primitive.Resource("plain", {
    boolean: plainBool,
    float: plainNumber,
    integer: plainInteger,
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
    float: secretNumber,
    integer: secretInteger,
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
