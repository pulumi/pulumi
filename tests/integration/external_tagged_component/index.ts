import * as pulumi from "@pulumi/pulumi";
import * as randomPluginComponent from "@pulumi/random-plugin-component";

let randomPetGenerator = new randomPluginComponent.RandomPetGenerator("pet", { length: 3, prefix: "test" });
let randomStringGenerator = new randomPluginComponent.RandomStringGenerator("string", { length: 8 });

export const randomPet = randomPetGenerator.pet;
export const randomString = randomStringGenerator.string;
