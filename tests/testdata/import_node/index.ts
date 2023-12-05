import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, opts: pulumi.ComponentResourceOptions) {
        super("pkg:index:MyComponent", name, {}, opts);

        new random.RandomPet("username", {}, { parent: this });
    }
}

const username = new random.RandomPet("username", {});

const component = new MyComponent("component", {});