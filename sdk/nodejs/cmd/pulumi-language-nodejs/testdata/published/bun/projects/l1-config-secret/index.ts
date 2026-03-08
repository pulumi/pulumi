import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aNumber = config.requireSecretNumber("aNumber");
export const roundtrip = aNumber;
export const theSecretNumber = aNumber.apply(aNumber => aNumber + 1.25);
