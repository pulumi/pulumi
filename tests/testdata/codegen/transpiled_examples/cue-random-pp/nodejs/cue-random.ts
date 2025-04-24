import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

const randomPassword = new random.RandomPassword("randomPassword", {
    length: 16,
    special: true,
    overrideSpecial: "_%@",
});
export const password = randomPassword.result;
