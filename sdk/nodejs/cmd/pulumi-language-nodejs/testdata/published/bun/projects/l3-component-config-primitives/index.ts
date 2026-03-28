import * as pulumi from "@pulumi/pulumi";
import { PrimitiveComponent } from "./primitiveComponent";

const config = new pulumi.Config();
const plainBool = config.requireBoolean("plainBool");
const plainNumber = config.requireNumber("plainNumber");
const plainString = config.require("plainString");
const secretBool = config.requireSecretBoolean("secretBool");
const secretNumber = config.requireSecretNumber("secretNumber");
const secretString = config.requireSecret("secretString");
const plain = new PrimitiveComponent("plain", {
    boolean: plainBool,
    float: plainNumber + 0.5,
    integer: plainNumber,
    string: plainString,
});
const secret = new PrimitiveComponent("secret", {
    boolean: secretBool,
    float: secretNumber.apply(secretNumber => secretNumber + 0.5),
    integer: secretNumber,
    string: secretString,
});
