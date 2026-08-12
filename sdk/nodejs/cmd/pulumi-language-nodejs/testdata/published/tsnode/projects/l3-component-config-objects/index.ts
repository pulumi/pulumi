import * as pulumi from "@pulumi/pulumi";
import { PrimitiveComponent } from "./primitiveComponent";

const config = new pulumi.Config();
const plainNumberArray = config.requireObject<Array<number>>("plainNumberArray");
const plainBooleanMap = config.requireObject<Record<string, boolean>>("plainBooleanMap");
const secretNumberArray = config.requireSecretObject<Array<number>>("secretNumberArray");
const secretBooleanMap = config.requireSecretObject<Record<string, boolean>>("secretBooleanMap");
const plain = new PrimitiveComponent("plain", {
    numberArray: plainNumberArray,
    booleanMap: plainBooleanMap,
});
const secret = new PrimitiveComponent("secret", {
    numberArray: secretNumberArray,
    booleanMap: secretBooleanMap,
});
