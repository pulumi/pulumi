import * as pulumi from "@pulumi/pulumi";

const myComponent = new pulumi.ComponentResource("my:custom:Component", "myComponent", {
    aNumber: 42,
    aString: "hello",
    aJson: JSON.stringify({
        key: "value",
    }),
});
