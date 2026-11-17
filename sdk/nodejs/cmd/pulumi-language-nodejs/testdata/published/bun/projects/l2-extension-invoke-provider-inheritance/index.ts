import * as pulumi from "@pulumi/pulumi";
import * as extbase from "@pulumi/extbase";
import * as myext from "@pulumi/myext";

export default async () => {
    const prov = new extbase.Provider("prov", {});
    const parent = new extbase.Base("parent", {}, {
        provider: prov,
    });
    // This should use prov from parent, because prov is the base provider of myext.
    const greeting = myext.greetOutput({
        name: "World",
    }, {
        parent: parent,
    });
    return {
        greeting: greeting.greeting,
    };
}
