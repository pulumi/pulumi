import * as random from "@pulumi/random";

const username = new random.RandomPet("username", {});

export const name = username.id