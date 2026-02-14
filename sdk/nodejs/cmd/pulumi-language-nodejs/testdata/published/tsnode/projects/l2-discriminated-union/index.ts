import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union from "@pulumi/discriminated-union";

const catExample = new discriminated_union.Example("catExample", {
    pet: {
        petType: "cat",
        meow: "meow",
    },
    pets: [{
        petType: "cat",
        meow: "purr",
    }],
});
const dogExample = new discriminated_union.Example("dogExample", {
    pet: {
        petType: "dog",
        bark: "woof",
    },
    pets: [
        {
            petType: "dog",
            bark: "bark",
        },
        {
            petType: "cat",
            meow: "hiss",
        },
    ],
});
