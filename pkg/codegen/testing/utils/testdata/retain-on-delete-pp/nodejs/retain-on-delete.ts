import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

const foo = new random.RandomPet("foo", {}, {
    retainOnDelete: true,
});
