import * as pulumi from "@pulumi/pulumi";
import { PrimitiveComponent } from "./primitiveComponent";

const config = new pulumi.Config();
const plainBool = config.requireBoolean("plainBool");
const plainNumber = config.requireNumber("plainNumber");
const plainInteger = config.requireNumber("plainInteger");
const plainString = config.require("plainString");
const secretBool = config.requireSecretBoolean("secretBool");
const secretNumber = config.requireSecretNumber("secretNumber");
const secretInteger = config.requireSecretNumber("secretInteger");
const secretString = config.requireSecret("secretString");
const plain = new PrimitiveComponent("plain", {
    boolean: plainBool,
    float: plainNumber,
    integer: plainInteger,
    string: plainString,
});
const secret = new PrimitiveComponent("secret", {
    boolean: secretBool,
    float: secretNumber,
    integer: secretInteger,
    string: secretString,
});
