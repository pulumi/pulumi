import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, opts: pulumi.ComponentResourceOptions) {
        super("pkg:index:MyComponent", name, {}, opts);

        new random.RandomPet("username", {}, { parent: this });
    }
}

const username = new random.RandomPet("username", {});

const component = new MyComponent("component", {
    // Add a dependency on the username resource to ensure it is created first. Depending on the order the
    // RandomPet resources are created the preview can generate different names for them. But our test expects
    // the first resource to be the renamed one.
    dependsOn: [username],
});