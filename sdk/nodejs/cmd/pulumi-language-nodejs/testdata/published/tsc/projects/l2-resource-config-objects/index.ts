import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const config = new pulumi.Config();
const plainNumberArray = config.requireObject<Array<number>>("plainNumberArray");
const plainBooleanMap = config.requireObject<Record<string, boolean>>("plainBooleanMap");
const secretNumberArray = config.requireSecretObject<Array<number>>("secretNumberArray");
const secretBooleanMap = config.requireSecretObject<Record<string, boolean>>("secretBooleanMap");
const plain = new primitive.Resource("plain", {
    boolean: true,
    float: 3.5,
    integer: 3,
    string: "plain",
    numberArray: plainNumberArray,
    booleanMap: plainBooleanMap,
});
const secret = new primitive.Resource("secret", {
    boolean: true,
    float: 3.5,
    integer: 3,
    string: "secret",
    numberArray: secretNumberArray,
    booleanMap: secretBooleanMap,
});
